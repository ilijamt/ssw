package ssw

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	wg     sync.WaitGroup
	logger *zap.Logger

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
		o.logger.Info("Attaching handler", zap.String("signal", signal.String()))
		o.handlers[signal] = fn
	}
}

// Start is a wrapper around Run
func (o *Service) Start() error {
	return o.Run()
}

// Stop is a wrapper around Close
func (o *Service) Stop() error {
	return o.Close()
}

// Run runs the service and waits for interruption signal, or the signal handlers
func (o *Service) Run() (err error) {
	o.logger.Info("Service started", zap.String("name", o.name))
	defer o.logger.Info("Service stopped", zap.String("name", o.name))
	if o.config.DelayStart > 0 {
		o.logger.Info("Delaying start", zap.String("delay", o.config.DelayStart.String()))
		time.Sleep(o.config.DelayStart)
	}

	o.closeMU.Lock()
	if o.closed {
		o.closeMU.Unlock()
		return errors.New(ErrServiceAlreadyRanOnce)
	}
	o.closeMU.Unlock()

	defer func() {
		delay := time.Duration(o.config.Timeout) * time.Millisecond
		o.logger.Info("Delaying shutdown", zap.Duration("delay", delay))
		time.Sleep(delay)
	}()

	// We run the handler for start
	if o.HandleStart != nil {
		o.logger.Info("Executing start handler")
		if err = o.HandleStart(); err != nil {
			o.logger.Error("Failed to execute start handler", zap.Error(err))
			return err
		}
	}

	// start all the clients we have in a go routine in case they are blocking
	for _, client := range o.clients {
		if c, ok := client.(ClientStart); ok {
			o.logger.Info("Client starting", zap.String("name", client.Name()))
			go func(name string, c ClientStart) {
				var err error
				if err = c.Start(); err != nil && o.HandleError != nil {
					o.HandleError(err)
				}
				defer o.logger.Info("Client started", zap.String("name", name), zap.Error(err))
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
	go func(signals []os.Signal, c chan interface{}, sig chan os.Signal, noSignals bool, tickDuration time.Duration) {
		defer o.wg.Done()

		// if there are no signals we don't need to do this
		if noSignals {
			o.logger.Info("No operating system signals available to bind to")
			return
		}

		o.logger.Info("Binding operating signals", zap.Reflect("signals", signals), zap.Duration("tick", tickDuration))
		ticker := time.NewTicker(tickDuration)
		signal.Notify(sig, signals...)

		for {
			select {
			case <-ticker.C:
				// we don't do anything here just in case, there are situation where
				// channel blocked occurs after a long time of inactivity, this should
				// hopefully prevent it
				o.logger.Debug("Tick")
			case s := <-sig:
				o.wg.Add(1)
				o.logger.Info("Operating system signal detected", zap.String("signal", s.String()))

				go func() { // trigger the correct handler if we have it
					defer o.wg.Done()
					o.handlerMU.Lock()
					defer o.handlerMU.Unlock()
					if fn, ok := o.handlers[s]; ok {
						o.logger.Info("Executing operating system signal handler", zap.String("signal", s.String()))
						var err error
						if err = fn(s); err != nil && o.HandleError != nil {
							o.HandleError(err)
						}
						o.logger.Info("Executed operating system signal handler", zap.String("signal", s.String()), zap.Error(err))
					}
				}()
			case <-c:
				return
			}
		}
	}(signals, chanClose, o.handlerSignal, !hasSignals, o.config.TickInterval)

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
				o.logger.Info("Client closing", zap.String("name", client.Name()))
				o.wg.Add(1) // if a client can be stopped only then add to the WaitGroup
				go func(name string, c ClientStop) {
					defer o.wg.Done()
					var err error
					if err = c.Stop(); err != nil && o.HandleError != nil {
						o.HandleError(err)
					}
					defer o.logger.Info("Client closed", zap.String("name", name), zap.Error(err))
				}(client.Name(), c)
			}
		}

		if o.HandleStop != nil {
			o.logger.Info("Executing stop handler")
			if err = o.HandleStop(); err != nil {
				o.logger.Error("Failed to execute stop handler", zap.Error(err))
			}
		}

	}(chanClose, o.handlerSignal, hasSignals)

	o.wg.Wait()
	o.close()

	return err
}

// SendSignal send a signal to the handler if we want to manually trigger a handler without waiting for it from the OS
func (o *Service) SendSignal(signal os.Signal) error {
	o.logger.Info("Sending signal", zap.String("signal", signal.String()))
	o.handlerSignal <- signal
	return nil
}

// Close notifies that the service wants to be closed
func (o *Service) Close() error {
	o.logger.Info("Service closing")
	defer o.logger.Info("Service closed")
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
		logger:             zap.NewNop(),
	}
	return svc
}

// WithLogger wraps the service with a logger
func WithLogger(service *Service, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	service.logger = logger
	logger.Info("Create a new service with logger", zap.Reflect("cfg", service.config))
	return service
}
