package engine

import (
	"encoding/json"
	"io"
	"net/http"
)

// The PreparationOptions structure contains the engine specific parts of the
// task payload, and other things an engine may need to prepare a sandbox.
type PreparationOptions struct {
	// The task.payload.start property is engine specific and we leave to the
	// engine implementor to parse it.
	Start json.RawMessage
	// The task.payload.options property is engine specific and we leave to the
	// engine implementor to parse it.
	Options json.RawMessage
}

// The StartOptions structure contains the cache folders, proxies and other
// things  an engine needs may need to start execution in a sandbox.
type StartOptions struct {
	// Mapping from identifier string to CacheFolder, the engine implementor may
	// assume that the CacheFolder was created with Engine.NewCacheFolder, hence,
	// it's safe to cast it to an internal type.
	//
	// The idea of cache folders is that some code inside the sandbox may cache
	// things between tasks, or access read-only caches that have been setup by
	// the runtime. This ensures that not everything has to be downloaded. As an
	// engine implementor you need not concern yourself with what is inside a
	// cache, when it is disposed or in what sandbox boxes it is mounted. You just
	// implement a CacheFolder abstraction, and ensure that it can be mounted in
	// your sandbox.
	//
	// Interpretation of the string identifier is left to the engine. Engine
	// specific documentation should reflect how it is used. The ideal solution
	// is obviously to let the identifier be a file path where the cache folder
	// is mounted. But not all platforms supports this, so you could also use it
	// as the name of environment variable that points to the mount point for
	// the CacheFolder.
	//
	// Either way interpretation is engine specific and must be documented by the
	// engine implementor. The implementor is free to restrict the format of the
	// string, in which StartSandbox() should return MalformedPayloadError if the
	// string violates the restrictions.
	//
	// If this engine doesn't support read-only and/or mutable caches it may
	// return ErrReadOnlyCacheNotSupported or ErrMutableCacheNotSupported from
	// StartSandbox() if ReadOnlyCaches or MutableCaches isn't empty.
	ReadOnlyCaches map[string]CacheFolder
	MutableCaches  map[string]CacheFolder
	// Mapping from identifier string to HTTP request handler for proxy.
	//
	// This idea of a proxy is that some code inside the sandbox may perform an
	// http request that is forwarded to the worker side. Here we may implement
	// different proxies, ranging from simply forwarding the requests to adding
	// authentication or transmitting the request to a special host on a VPN.
	// As an engine implementor it's not really your concern what happens to the
	// request, merely that the request gets to the http.Handler.
	//
	// Interpretation of string identifier is left ot the engine. Engine speific
	// documentation should reflect how it's used. The ideal solution is obviously
	// to let the string be a hostname. However, some platforms may not support
	// this, and an alternative may be to let the identifier be a environment
	// variable containing IP and port, or some other way to submit an HTTP
	// request to the proxy.
	//
	// Engines that doesn't support proxies may return ErrProxiesNotSupported
	// from StartSandbox() if this mapping isn't empty.
	Proxies map[string]http.Handler
}

// The PreparedSandbox interface wraps the state and logic required to execute a task
// and engines. This is naturally stateful, but focuses exclusively on execution
// of the task, whereas the runtime and worker code deals with state of the
// task, and tracking generic resources like CacheFolders.
//
// Granted an Execution interface may still need to track Engine specific
// resources like docker images.
type PreparedSandbox interface {
	StartSandbox(options *ExecutionOptions) (Sandbox, error)
	Abort() error
}

type Shell interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Terminate() error
	Wait() bool
}

type Sandbox interface {
	Wait()
	Results() (ResultSet, error)
	NewShell() (Shell, error)
	Abort() error
}

// An ArtifactReader contains logic to read an artifact and know it's file path
type ArtifactReader struct {
	// Read/Close the file stream
	io.ReadCloser
	// Path to the artifact within the Sandbox
	Path string
}

type ResultSet interface {
	Success() bool
	ExtractFolder(path string) (<-chan (ArtifactReader), error)
	ExtractFile(path string) (ArtifactReader, error)
	ArchiveSandbox() (io.ReadCloser, error)
	Dispose() error
}
