package runtime

import (
	"net/url"

	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// Environment is a collection of objects that makes up a runtime environment.
//
// This type is intended to be passed by value, and should only contain pointers
// and interfaces for that reason.
type Environment struct {
	GarbageCollector gc.ResourceTracker
	TemporaryStorage
	webhookserver.WebHookServer // Optional, may be nil if not available
	Monitor
	Worker        Stoppable
	ProvisionerID string
	WorkerType    string
	WorkerGroup   string
	WorkerID      string
	RootURL       *url.URL
}

// GetServiceURL takes a service name and returns the full taskcluster
// URL of it.
func GetServiceURL(rootURL *url.URL, serviceName string) string {
	copyURL, err := url.Parse(rootURL.String())
	if err != nil {
		panic("WAT???")
	}
	copyURL.Host = serviceName + "." + copyURL.Host
	return copyURL.String()
}

// GetServiceURL returns the URL of the given service
func (env *Environment) GetServiceURL(serviceName string) string {
	return GetServiceURL(env.RootURL, serviceName)
}
