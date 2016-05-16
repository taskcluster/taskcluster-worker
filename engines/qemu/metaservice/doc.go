// Package metaservice implements the meta-data service that the guests use
// to talk to the host.
//
// The meta-data service is exposed to the guest on 169.254.169.254:80.
// This is how the command and environment variables enter the virtual machine.
// It is also the services that the guest uses to report logs and final result.
package metaservice
