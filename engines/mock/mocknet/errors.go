package mocknet

import "errors"

// ErrListenerClosed is returned when a listener have been closed
var ErrListenerClosed = errors.New("Listener have been closed!")

// ErrAddressInUse is returned if the address already is in use
var ErrAddressInUse = errors.New("Address is already in use by another listener")

// ErrConnRefused is returned if the connection is refused
var ErrConnRefused = errors.New("Connection refused, no listener for the given address")
