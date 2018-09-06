package ssw

import (
	"github.com/pkg/errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ErrServiceAlreadyRanOnce tells us that the service already ran and you cannot run it again, if you need to run it again you need to create a new objecct
const ErrServiceAlreadyRanOnce = "service already ran once, cannot run again"

var terminationSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
}

// Service represents a service handling requests, what it does it bootstraps the process and wraps it in a nice package where you don't have to worry about starting/stopping the services
type Service struct {
	wg sync.WaitGroup

	closed             bool
	name               string
	config             Config
	details            *Version
	terminationSignals []os.Signal

	// termination handlers
	interrupt   chan os.Signal
	HandleStart HandlerGeneric
	HandleStop  HandlerGeneric
	HandleError HandlerError

	handlerMU sync.Mutex
	closeMU   sync.Mutex

	// used for registering handler on specific signals so you can easily run actions, like on SIGHUP
	handlerSignal chan os.Signal
	handlers      map[os.Signal]HandlerInterrupt

	// self registration
	clients []Client
}

// Handler add a specific function that will run when the signal is detected, which function will not initiate the stopping of the application, so it can be triggered as many times as necessary
func (o *Service) Handler(fn HandlerInterrupt, signals ...os.Signal) {
	o.handlerMU.Lock()
	defer o.handlerMU.Unlock()
	for _, signal := range signals {
		o.handlers[signal] = fn
	}
}

// Run runs the service and waits for interruption signal, or the signal handlers
func (o *Service) Run() (err error) {
	o.closeMU.Lock()
	if o.closed {
		o.closeMU.Unlock()
		return errors.New(ErrServiceAlreadyRanOnce)
	}
	o.closeMU.Unlock()

	defer time.Sleep(time.Duration(o.config.Timeout) * time.Millisecond)

	// We run the handler for start
	if o.HandleStart != nil {
		if err = o.HandleStart(); err != nil {
			return err
		}
	}

	// start all the clients we have in a go routine in case they are blocking
	for _, client := range o.clients {
		if c, ok := client.(ClientStart); ok {
			go func(name string, c ClientStart) {
				if err := c.Start(); err != nil && o.HandleError != nil {
					o.HandleError(err)
				}
			}(client.Name(), c)
		}
	}

	// we use this to stop the signal handlers
	o.handlerMU.Lock()
	chanClose := make(chan interface{})
	signals := make([]os.Signal, 0, len(o.handlers))
	for sig := range o.handlers {
		signals = append(signals, sig)
	}
	hasSignals := len(signals) > 0
	o.handlerMU.Unlock()

	o.wg.Add(1)
	go func(signals []os.Signal, c chan interface{}, sig chan os.Signal, noSignals bool) {
		defer o.wg.Done()

		// if there are no signals we don't need to do this
		if noSignals {
			return
		}

		ticker := time.NewTicker(time.Second)
		signal.Notify(sig, signals...)

		for {
			select {
			case <-ticker.C:
				// we don't do anything here just in case, there are situation where
				// channel blocked occurs after a long time of inactivity, this should
				// hopefully prevent it
			case s := <-sig:
				o.wg.Add(1)

				go func() { // trigger the correct handler if we have it
					defer o.wg.Done()
					o.handlerMU.Lock()
					defer o.handlerMU.Unlock()
					if fn, ok := o.handlers[s]; ok {
						if err := fn(s); err != nil && o.HandleError != nil {
							o.HandleError(err)
						}
					}
				}()
			case <-c:
				return
			}
		}
	}(signals, chanClose, o.handlerSignal, !hasSignals)

	// Run for the interrupt before we continue the flow
	o.wg.Add(1)
	go func(c chan interface{}, sig chan os.Signal, hasSignals bool) {
		defer o.wg.Done()

		signal.Notify(o.interrupt, o.terminationSignals...)
		s := <-o.interrupt // we wait on the termination signal

		// if we have handler running close them
		if hasSignals {
			sig <- s // forward the termination signal in case we are also listening on that one
			close(c) // so we also close signal handlers
		}

		for _, client := range o.clients {
			if c, ok := client.(ClientStop); ok {
				o.wg.Add(1) // if a client can be stopped only then add to the WaitGroup
				go func(name string, c ClientStop) {
					defer o.wg.Done()

					if err := c.Stop(); err != nil && o.HandleError != nil {
						o.HandleError(err)
						return
					}
				}(client.Name(), c)
			}
		}

		if o.HandleStop != nil {
			err = o.HandleStop()
		}

	}(chanClose, o.handlerSignal, hasSignals)

	o.wg.Wait()
	o.close()

	return err
}

// SendSignal send a signal to the handler if we want to manually trigger a handler without waiting for it from the OS
func (o *Service) SendSignal(signal os.Signal) error {
	o.handlerSignal <- signal
	return nil
}

// Close notifies that the service wants to be closed
func (o *Service) Close() error {
	o.closeMU.Lock()
	o.closed = true
	o.interrupt <- syscall.SIGTERM
	o.closeMU.Unlock()
	return nil
}

func (o *Service) close() error {
	close(o.interrupt)
	return nil
}

// New creates a new instance of service with provided configuration and details
func New(name string, cfg Config, details *Version, clients ...Client) *Service {
	svc := &Service{
		config:             cfg,
		details:            details,
		name:               name,
		interrupt:          make(chan os.Signal, 1),
		handlerSignal:      make(chan os.Signal, 1),
		clients:            clients,
		handlers:           make(map[os.Signal]HandlerInterrupt),
		handlerMU:          sync.Mutex{},
		closeMU:            sync.Mutex{},
		terminationSignals: terminationSignals,
	}
	if cfg.DelayStart > 0 {
		time.Sleep(cfg.DelayStart)
	}
	return svc
}
