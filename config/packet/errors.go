package configpacket

import "errors"

// ErrNoPublicIPv4Address is returned if the instance doesn't have a public ipv4 address
var ErrNoPublicIPv4Address = errors.New("Metadata for this instances doesn't have any public IPv4 address")

// ErrNoPublicIPv6Address is returned if the instance doesn't have a public ipv6 address
var ErrNoPublicIPv6Address = errors.New("Metadata for this instances doesn't have any public IPv6 address")
