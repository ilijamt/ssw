package ssw

import "os"

// HandlerGeneric is a generic function that will be executed at various points
type HandlerGeneric func() error

// HandlerInterrupt is a generic function that is executed on a specific signal
type HandlerInterrupt func(signal os.Signal) error

// HandlerError is a function that will get called if there is an error, here you can decide if you want to terminate the application or not
type HandlerError func(err error) error
