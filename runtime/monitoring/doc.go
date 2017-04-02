// Package monitoring provides multiple implementations of runtime.Monitor.
//
// In addition to supplying runtime.Monitor implementations this package also
// provides and a ConfigSchema and a generic New(config) method that can be
// used to instantiate one of the implementations dependening on configuration.
// This allows for configurable selection of monitoring strategy without
// complicating the application with configuration.
package monitoring
