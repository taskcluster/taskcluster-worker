//go:generate interfacer -for "\"github.com/taskcluster/taskcluster-client-go/queue\".Queue" -as client.Queue -o queue.go
//go:generate interfacer -for "\"github.com/taskcluster/taskcluster-client-go/auth\".Auth" -as client.Auth -o auth.go
//go:generate interfacer -for "\"github.com/taskcluster/taskcluster-client-go/index\".Index" -as client.Index -o index.go
//go:generate interfacer -for "\"github.com/taskcluster/taskcluster-client-go/secrets\".Secrets" -as client.Secrets -o secrets.go
//go:generate mockery -inpkg -name Queue -output .
//go:generate mockery -inpkg -name Auth -output .
//go:generate mockery -inpkg -name Index -output .
//go:generate mockery -inpkg -name Secrets -output .

// Package client contains interfaces wrapping taskcluster-client-go and mock
// implementations of these interfaces.
package client
