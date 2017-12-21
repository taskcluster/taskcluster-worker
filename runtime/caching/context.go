package caching

import "context"

// Context for creation of resources
type Context interface {
	context.Context
	Progress(description string, percent float64)
}
