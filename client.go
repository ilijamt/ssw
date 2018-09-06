package ssw

// Client generic interface
type Client interface {
	Name() string
}

// ClientStart for clients that are able to start
type ClientStart interface {
	Start() error
}

// ClientStop for clients that are able to start, the client should make sure that the stop is blocking and it will only continue after it has successfully shutdown
type ClientStop interface {
	Stop() error
}
