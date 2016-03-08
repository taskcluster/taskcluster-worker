package runtime

// ShutdownManager implements a method for listening for shutdown events.  Consumers
type ShutdownManager interface {
	WaitForShutdown() <-chan struct{}
}

// NewShutdownManager will return a shutdown manager appropriate for the host that
// the worker is being run on.
//
// Shutdown events are triggered different ways depending on where the worker is running.
// When running in AWS, then notifications are sent on their meta-data api, but running
// locally could cause the worker to represent to different kind of shutdown events.
func NewShutdownManager(host string) ShutdownManager {
	var manager ShutdownManager
	switch host {
	case "AWS":
		manager = &AWSShutdownManager{}
	case "local":
		manager = &LocalShutdownManager{}
	}

	return manager
}

// AWSShutdownManager is a ShutdownManager that will listen for shutdowns on the notification
// api provided by AWS.
type AWSShutdownManager struct{}

// WaitForShutdown will listen for notification events from the AWS shutdown endpoint
// and close the channel when a shutdown notification is received.
// When a shutdown event is received, shutdown ch
func (AWSShutdownManager) WaitForShutdown() <-chan struct{} {
	c := make(chan struct{})
	return c
}

// LocalShutdownManager simple ShutdownManager that could listen for shutdown events
// suitable for a local non-cloud environment (such as SIGTERM).
type LocalShutdownManager struct{}

// WaitForShutdown will listen for local events to signify a worker shutdown
func (LocalShutdownManager) WaitForShutdown() <-chan struct{} {
	c := make(chan struct{})
	return c
}
