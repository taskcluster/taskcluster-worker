package mocknet

import "errors"

// ErrListenerClosed is returned when a listener has been closed
var ErrListenerClosed = errors.New("listener has been closed")

// ErrAddressInUse is returned if the address already is in use
var ErrAddressInUse = errors.New("address is already in use by another listener")

// ErrConnRefused is returned if the connection is refused
var ErrConnRefused = errors.New("connection refused, no listener for the given address")
