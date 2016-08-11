package mockengine

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

type mount struct {
	volume   *volume
	readOnly bool
}

// In this example it is easier to just implement with one object.
// This way we won't have to pass data between different instances.
// In larger more complex engines that downloads stuff, etc. it's probably not
// a good idea to implement everything in one structure.
type sandbox struct {
	sync.Mutex
	engines.SandboxBuilderBase
	engines.SandboxBase
	engines.ResultSetBase
	payload *payload
	context *runtime.TaskContext
	env     map[string]string
	mounts  map[string]*mount
	proxies map[string]http.Handler
	files   map[string][]byte
	result  bool
	done    chan struct{}
}

///////////////////////////// Implementation of SandboxBuilder interface

func (s *sandbox) StartSandbox() (engines.Sandbox, error) {
	s.Lock()
	defer s.Unlock()
	s.done = make(chan struct{})
	return s, nil
}

func (s *sandbox) AttachVolume(mountpoint string, v engines.Volume, readOnly bool) error {
	// We can type cast Volume to our internal type as we know the volume was
	// created by NewCacheFolder() or NewMemoryDisk(), this is a contract.
	vol, valid := v.(*volume)
	if !valid {
		// TODO: Write to some sort of log if the type assertion fails
		return engines.ErrContractViolation
	}
	// Lock before we access mounts as this method may be called concurrently
	s.Lock()
	defer s.Unlock()
	if strings.ContainsAny(mountpoint, " ") {
		return engines.NewMalformedPayloadError("MockEngine mountpoints cannot contain space")
	}
	if s.mounts[mountpoint] != nil {
		return engines.ErrNamingConflict
	}
	s.mounts[mountpoint] = &mount{
		volume:   vol,
		readOnly: readOnly,
	}
	return nil
}

func (s *sandbox) AttachProxy(name string, handler http.Handler) error {
	// Lock before we access proxies as this method may be called concurrently
	s.Lock()
	defer s.Unlock()
	if strings.ContainsAny(name, " ") {
		return engines.NewMalformedPayloadError(
			"MockEngine proxy names cannot contain space.",
			"Was given proxy name: '", name, "' which isn't allowed!",
		)
	}
	if s.proxies[name] != nil {
		return engines.ErrNamingConflict
	}
	s.proxies[name] = handler
	return nil
}

func (s *sandbox) SetEnvironmentVariable(name string, value string) error {
	s.Lock()
	defer s.Unlock()
	if strings.Contains(name, " ") {
		return engines.NewMalformedPayloadError(
			"MockEngine environment variable names cannot contain space.",
			"Was given environment variable name: '", name, "' which isn't allowed!",
		)
	}
	if _, ok := s.env[name]; ok {
		return engines.ErrNamingConflict
	}
	s.env[name] = value
	return nil
}

///////////////////////////// Implementation of Sandbox interface

// List of functions implementing the task.payload.start.function functionality.
var functions = map[string]func(*sandbox, string) bool{
	"true":  func(s *sandbox, arg string) bool { return true },
	"false": func(s *sandbox, arg string) bool { return false },
	"set-volume": func(s *sandbox, arg string) bool {
		mount := s.mounts[arg]
		if mount == nil || mount.readOnly {
			return false
		}
		mount.volume.value = true
		return true
	},
	"get-volume": func(s *sandbox, arg string) bool {
		mount := s.mounts[arg]
		if mount == nil {
			return false
		}
		return mount.volume.value
	},
	"ping-proxy": func(s *sandbox, arg string) bool {
		u, err := url.Parse(arg)
		if err != nil {
			s.context.Log("Failed to parse url: ", arg, " got error: ", err)
			return false
		}
		handler := s.proxies[u.Host]
		if handler == nil {
			s.context.Log("No proxy for hostname: ", u.Host, " in: ", arg)
			return false
		}
		// Make a fake HTTP request and http response recorder
		s.context.Log("Pinging")
		req, err := http.NewRequest("GET", arg, nil)
		if err != nil {
			panic(err)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		// Log response
		s.context.Log(w.Body.String())
		return w.Code == http.StatusOK
	},
	"write-log": func(s *sandbox, arg string) bool {
		s.context.Log(arg)
		return true
	},
	"write-error-log": func(s *sandbox, arg string) bool {
		s.context.Log(arg)
		return false
	},
	"write-files": func(s *sandbox, arg string) bool {
		for _, path := range strings.Split(arg, " ") {
			s.files[path] = []byte("Hello World")
		}
		return true
	},
	"print-env-var": func(s *sandbox, arg string) bool {
		val, ok := s.env[arg]
		s.context.Log(val)
		return ok
	},
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	// No need to lock access to payload, as it can't be mutated at this point
	select {
	case <-s.done:
		s.result = false
		return s, errors.New("Task execution has been aborted")
	case <-time.After(time.Duration(s.payload.Delay) * time.Millisecond):
		// No need to lock access mounts and proxies either
		f := functions[s.payload.Function]
		if f == nil {
			return nil, engines.NewMalformedPayloadError("Unknown function")
		}
		result := f(s, s.payload.Argument)
		s.Lock()
		defer s.Unlock()
		s.result = result
		return s, nil
	}
}

func (s *sandbox) Abort() error {
	close(s.done)
	return nil
}

func (s *sandbox) NewShell(command []string, tty bool) (engines.Shell, error) {
	if len(command) > 0 || tty {
		return nil, engines.ErrFeatureNotSupported
	}
	return newShell(), nil
}

func (s *sandbox) ListDisplays() ([]engines.Display, error) {
	return []engines.Display{
		{
			Name:        "MockDisplay",
			Description: "Simple mock VNC display rendering a static test image",
			Width:       mockDisplayWidth,
			Height:      mockDisplayHeight,
		},
	}, nil
}

func (s *sandbox) OpenDisplay(name string) (io.ReadWriteCloser, error) {
	if name != "MockDisplay" {
		return nil, engines.ErrNoSuchDisplay
	}
	return newMockDisplay(), nil
}

///////////////////////////// Implementation of ResultSet interface

func (s *sandbox) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	data := s.files[path]
	if len(data) == 0 {
		return nil, engines.ErrResourceNotFound
	}
	return ioext.NopCloser(bytes.NewReader(data)), nil
}

func (s *sandbox) ExtractFolder(folder string, handler engines.FileHandler) error {
	if !strings.HasSuffix(folder, "/") {
		folder += "/"
	}
	wg := sync.WaitGroup{}
	m := sync.Mutex{}
	handlerError := false
	foundFolder := false
	for path, data := range s.files {
		if strings.HasPrefix(path, folder) {
			foundFolder = true
			if path == folder {
				// In this engine a filename ending with / is a folder, and its content
				// is ignored, it's only used as indicator of folder existence
				continue
			}
			wg.Add(1)
			go func(path string, data []byte) {
				err := handler(path, ioext.NopCloser(bytes.NewReader(data)))
				if err != nil {
					m.Lock()
					handlerError = true
					m.Unlock()
				}
				wg.Done()
			}(path, data)
		}
	}
	wg.Wait()
	if !foundFolder {
		return engines.ErrResourceNotFound
	}
	if handlerError {
		return engines.ErrHandlerInterrupt
	}
	return nil
}

func (s *sandbox) Success() bool {
	// No need to lock access as result is immutable
	return s.result
}
