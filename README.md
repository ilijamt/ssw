Simple Service Wrapper
======================

[![Go Report Card](https://goreportcard.com/badge/github.com/ilijamt/ssw)](https://goreportcard.com/report/github.com/ilijamt/ssw) [![Build Status](https://travis-ci.org/ilijamt/ssw.svg?branch=master)](https://travis-ci.org/ilijamt/ssw) [![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/ilijamt/ssw/blob/master/LICENSE)

Serves as a wrapper for simple service where full systemd integration is not needed.

It provides some basic functionality out of the box

* Start/Stop of the application
* Allows you to do action based on signals from the OS, like reloading configuration or activating/deactivating new functionality
* Allows you to start and stop services if the implement the ClientStart and/or ClientStop interfaces

```go
package main

import (
	"fmt"
	"os"
	"syscall"
	"github.com/ilijamt/ssw"
)

func main() {

	svc := ssw.New("Test", ssw.NewConfig(), ssw.NewVersion("Test", "Desc", "Ver", "Hash", "Date", "Clean"))

	svc.HandleStart = func() error {
		fmt.Println("Handle start")
		return nil
	}

	svc.HandleStop = func() error {
		fmt.Println("Handle stop")
		return nil
	}
	
	svc.HandleError = func(err error) error {
        fmt.Println("Handle error")
		return nil
	}
	
	handler := func(signal os.Signal) error {
		fmt.Printf("We have a %s, we can do whatever we want here", signal)
		return nil
	}

	svc.Handler(handler, syscall.SIGHUP, syscall.SIGUSR1)

	// This will start the service and register all handlers and everything
	if err := svc.Run(); err != nil {
		panic(err)
	}
}

```

