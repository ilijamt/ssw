package ssw_test

import (
	"errors"
	"fmt"
	"github.com/ilijamt/envwrap"
	"github.com/ilijamt/ssw"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"os"
	"syscall"
	"testing"
	"time"
)

type dummyClient struct {
	isRunning *atomic.Bool
	errStart  error
	errStop   error
	name      string
}

func newDummyClient(name string, start bool, stop bool) *dummyClient {
	obj := &dummyClient{
		name:      name,
		isRunning: atomic.NewBool(false),
	}
	if start {
		obj.errStart = errors.New("failed to start")
	}
	if stop {
		obj.errStop = errors.New("failed to stop")
	}
	return obj
}

func (o *dummyClient) Name() string {
	return o.name
}

func (o *dummyClient) Start() error {
	if o.errStart == nil {
		o.isRunning.Store(true)
	} else {
		o.isRunning.Store(false)
	}
	return o.errStart
}

func (o *dummyClient) Stop() error {
	if o.errStop != nil {
		o.isRunning.Store(true)
	} else {
		o.isRunning.Store(false)
	}
	return o.errStop
}

func Test_NewService(t *testing.T) {
	cfg := ssw.NewConfig()
	version := ssw.NewVersion("name", "desc", "ver", "hash", "date", "clean")
	svc := ssw.WithLogger(
		ssw.New("test", cfg, version),
		nil,
	)

	assert.NotEmpty(t, version.GetDetails())
	assert.NotEmpty(t, version.GetVersion())
	assert.NotNil(t, svc)
	time.AfterFunc(time.Second, func() {
		svc.Close()
	})
	assert.NoError(t, svc.Run())
	assert.EqualError(t, svc.Run(), ssw.ErrServiceAlreadyRanOnce)
}

func Test_NewServiceFailToStart(t *testing.T) {
	cfg := ssw.NewConfig()
	version := ssw.NewVersion("name", "desc", "ver", "hash", "date", "clean")
	svc := ssw.WithLogger(
		ssw.New("test", cfg, version),
		zap.NewNop(),
	)
	svc.HandleStart = func() error {
		return errors.New("fail to start")
	}
	assert.NotEmpty(t, version.GetDetails())
	assert.NotEmpty(t, version.GetVersion())
	assert.NotNil(t, svc)
	time.AfterFunc(time.Second, func() {
		svc.Close()
	})
	assert.Error(t, svc.Run())
}

func Test_NewServiceFailToStop(t *testing.T) {
	cfg := ssw.NewConfig()
	version := ssw.NewVersion("name", "desc", "ver", "hash", "date", "clean")
	svc := ssw.WithLogger(
		ssw.New("test", cfg, version),
		zap.NewNop(),
	)
	svc.HandleStop = func() error {
		return errors.New("fail to stop")
	}
	assert.NotEmpty(t, version.GetDetails())
	assert.NotEmpty(t, version.GetVersion())
	assert.NotNil(t, svc)
	time.AfterFunc(time.Second, func() {
		svc.Close()
	})
	assert.Error(t, svc.Run())
}

func Test_Service_WaitAndHandlers(t *testing.T) {
	env := envwrap.NewStorage()
	defer env.ReleaseAll()

	env.Store("SERVICE_DELAY_START", "1")

	done := make(chan bool)

	handlerStart := false
	handlerStop := false

	cfg := ssw.NewConfig()
	cfg.DelayStart = time.Microsecond
	dummyClientOK := newDummyClient("dummyClientOK", false, false)
	assert.NotNil(t, dummyClientOK)
	assert.False(t, dummyClientOK.isRunning.Load())

	dummyClientFailStart := newDummyClient("dummyClientFailStart", true, false)
	assert.NotNil(t, dummyClientFailStart)
	assert.False(t, dummyClientFailStart.isRunning.Load())

	dummyClientFailStop := newDummyClient("dummyClientFailStop", false, true)
	assert.NotNil(t, dummyClientFailStop)
	assert.False(t, dummyClientFailStop.isRunning.Load())

	svc := ssw.WithLogger(
		ssw.New("test", cfg, ssw.NewVersion("name", "desc", "ver", "hash", "date", "clean"), dummyClientOK, dummyClientFailStart, dummyClientFailStop),
		zap.NewNop(),
	)
	svc.HandleStart = func() error {
		handlerStart = true
		return nil
	}

	calledSuccessfulHandler := atomic.NewBool(false)
	calledFailedHandler := atomic.NewUint32(0)

	svc.Handler(func(signal os.Signal) error {
		calledSuccessfulHandler.Store(true)
		return nil
	}, syscall.SIGTERM)

	svc.Handler(func(signal os.Signal) error {
		calledFailedHandler.Inc()
		return errors.New("failed to handle the request")
	}, syscall.SIGHUP)

	svc.HandleError = func(err error) error {
		assert.True(t, true)
		return err
	}

	svc.HandleStop = func() error {
		handlerStop = true
		return nil
	}

	assert.False(t, handlerStop)
	assert.False(t, handlerStart)

	assert.False(t, dummyClientOK.isRunning.Load())
	assert.False(t, dummyClientFailStart.isRunning.Load())
	assert.False(t, dummyClientFailStop.isRunning.Load())

	// verify we have stopped
	go func() {
		assert.NoError(t, svc.Start())
		assert.False(t, dummyClientOK.isRunning.Load())
		assert.False(t, dummyClientFailStart.isRunning.Load())
		assert.True(t, dummyClientFailStop.isRunning.Load())
		assert.True(t, calledSuccessfulHandler.Load())
		assert.EqualValues(t, 1, calledFailedHandler.Load())
		assert.True(t, handlerStart)
		assert.True(t, handlerStop)
		done <- true
	}()

	// verify stuff is running, after some time
	time.AfterFunc(time.Second, func() {
		assert.True(t, dummyClientOK.isRunning.Load())
		assert.False(t, dummyClientFailStart.isRunning.Load())
		assert.True(t, dummyClientFailStop.isRunning.Load())
		svc.SendSignal(syscall.SIGHUP)
	})

	time.AfterFunc(5*time.Second, func() {
		svc.Stop()
	})

	<-done

}

func ExampleNew() {

	cfg := ssw.NewConfig()

	svc := ssw.WithLogger(
		ssw.New("Test", cfg, ssw.NewVersion("Test", "Desc", "Ver", "Hash", "Date", "Clean")),
		zap.NewNop(),
	)

	svc.HandleStart = func() error {
		fmt.Println("Handle start")
		return nil
	}

	svc.HandleStop = func() error {
		fmt.Println("Handle stop")
		return nil
	}

	svc.HandleError = func(err error) error {
		return err
	}

	handler := func(signal os.Signal) error {
		fmt.Printf("We have a %s, we can do whatever we want here", signal)
		return nil
	}

	svc.Handler(handler, syscall.SIGHUP, syscall.SIGUSR1)

	// This will start the service and register all handlers and everything
	svc.Run()
}
