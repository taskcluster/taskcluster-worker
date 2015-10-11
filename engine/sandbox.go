package engine

import (
	"encoding/json"
	"io"
	"net/http"
)

// The SandboxPayload structure contains the engine specific parts of the
// task payload.
type SandboxPayload struct {
	Start   json.RawMessage
	Options json.RawMessage
}

// An ArtifactReader contains logic to read an artifact and know it's file path
type ArtifactReader struct {
	// Read/Close the file stream
	io.ReadCloser
	// Path to the artifact within the Sandbox
	Path string
}

// The Sandbox interface wraps the state and logic required to execute a task
// and engines. This is naturally stateful, but focuses exclusively on execution
// of the task, whereas the runtime and worker code deals with state of the
// task, and tracking generic resources like CacheFolders.
//
// Granted an Execution interface may still need to track Engine specific
// resources like docker images.
type Sandbox interface {
	AttachMutableCache(cache CacheFolder, target string) error
	AttachReadOnlyCache(cache CacheFolder, target string) error
	AttachProxy(name string, handler http.Handler) error
	// Execute the task, returns true if task exited zero, and false if it exited
	// non-zero.
	//
	// Notice that methods AttachMutableCache, AttachReadOnlyCache and AttachProxy
	// cannot be called after Execute().
	//
	// The Execute method is blocking
	Execute() (bool, error)
	NewShell() (Shell, error)
	Stop() error
	ExtractFolder(path string) <-chan (ArtifactReader)
	ExtractFile(path string) ArtifactReader
	ArchiveSandbox() io.ReadCloser
	Close() error
	/*
		//NewShell() Shell
		   AttachCache(source string, string target, readOnly bool) err
		   AttachProxy(name string, handler func(ResponseWriter, *Request)) err
		   NewShell() Shell
		   ArchiveFolder(path) <-chan(string, io.ReadCloser)
		   ArchiveFile(path) string, io.ReadCloser
		   ArchiveSandbox() io.ReadCloser

		   // TODO: Figure out how to report async errors, abort and differ between internal error
		   // and malformed-payload
		   // TODO: Figure out how to configure cache interaction
		   AttachService(image string, command string[], env) err
		   Start(command string[], env map[string]string) bool, err
		   StdinPipe() io.WriteCloser, err
		   StdoutPipe() io.ReadCloser, err
		   StderrPipe() io.ReadCloser, err
		   Abort()
	*/
}
