package mockengine

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
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
	environment runtime.Environment
	payload     payloadType
	context     *runtime.TaskContext
	env         map[string]string
	mounts      map[string]*mount
	proxies     map[string]http.Handler
	files       map[string][]byte
	sessions    atomics.WaitGroup
	shells      []engines.Shell
	displays    []io.ReadWriteCloser
	resolve     atomics.Once
	result      bool
	resultErr   error
	abortErr    error
}

///////////////////////////// Implementation of SandboxBuilder interface

func (s *sandbox) abortSessions() {
	s.Lock()
	defer s.Unlock()

	s.sessions.Drain()
	for _, shell := range s.shells {
		shell.Abort()
	}
	for _, display := range s.displays {
		display.Close()
	}
}

func (s *sandbox) StartSandbox() (engines.Sandbox, error) {
	s.Lock()
	defer s.Unlock()

	go func() {
		// No need to lock access to payload, as it can't be mutated at this point
		time.Sleep(time.Duration(s.payload.Delay) * time.Millisecond)
		// No need to lock access mounts and proxies either
		f := functions[s.payload.Function]
		var err error
		var result bool
		if f == nil {
			err = runtime.NewMalformedPayloadError("Unknown function")
		} else {
			result, err = f(s, s.payload.Argument)
		}
		s.sessions.WaitAndDrain()
		s.resolve.Do(func() {
			s.result = result
			s.resultErr = err
			s.abortErr = engines.ErrSandboxTerminated
		})
	}()

	return s, nil
}

func (s *sandbox) AttachVolume(mountpoint string, v engines.Volume, readOnly bool) error {
	// We can type cast Volume to our internal type as we know the volume was
	// created by NewCacheFolder() or NewMemoryDisk(), this is a contract.
	vol, valid := v.(*volume)
	if !valid {
		// TODO: Write to some sort of log if the type assertion fails
		return fmt.Errorf("invalid volume type")
	}
	// Lock before we access mounts as this method may be called concurrently
	s.Lock()
	defer s.Unlock()
	if strings.ContainsAny(mountpoint, " ") {
		return runtime.NewMalformedPayloadError("MockEngine mountpoints cannot contain space")
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
		return runtime.NewMalformedPayloadError(
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
		return runtime.NewMalformedPayloadError(
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
var functions = map[string]func(*sandbox, string) (bool, error){
	"true":  func(s *sandbox, arg string) (bool, error) { return true, nil },
	"false": func(s *sandbox, arg string) (bool, error) { return false, nil },
	"write-volume": func(s *sandbox, arg string) (bool, error) {
		// Parse arg as: <mountPoint>/<file_name>:<file_data>
		args := strings.SplitN(arg, "/", 2)
		volume_name := args[0]
		args = strings.SplitN(args[1], ":", 2)
		file_name := args[0]
		file_data := args[1]
		mount := s.mounts[volume_name]
		if mount == nil || mount.readOnly {
			return false, nil
		}
		mount.volume.files[file_name] = file_data
		return true, nil
	},
	"read-volume": func(s *sandbox, arg string) (bool, error) {
		// Parse arg as: <mountPoint>/<file_name>
		args := strings.SplitN(arg, "/", 2)
		volume_name := args[0]
		file_name := args[1]

		mount := s.mounts[volume_name]
		if mount == nil {
			return false, nil
		}
		s.context.Log(mount.volume.files[file_name])
		return mount.volume.files[file_name] != "", nil
	},
	"ping-proxy": func(s *sandbox, arg string) (bool, error) {
		u, err := url.Parse(arg)
		if err != nil {
			s.context.Log("Failed to parse url: ", arg, " got error: ", err)
			return false, nil
		}
		handler := s.proxies[u.Host]
		if handler == nil {
			s.context.Log("No proxy for hostname: ", u.Host, " in: ", arg)
			return false, nil
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
		return w.Code == http.StatusOK, nil
	},
	"write-log": func(s *sandbox, arg string) (bool, error) {
		s.context.Log(arg)
		return true, nil
	},
	"write-error-log": func(s *sandbox, arg string) (bool, error) {
		s.context.Log(arg)
		return false, nil
	},
	"write-log-sleep": func(s *sandbox, arg string) (bool, error) {
		s.context.Log(arg)
		time.Sleep(500 * time.Millisecond)
		return true, nil
	},
	"write-files": func(s *sandbox, arg string) (bool, error) {
		for _, path := range strings.Split(arg, " ") {
			s.files[path] = []byte("Hello World")
		}
		return true, nil
	},
	"print-env-var": func(s *sandbox, arg string) (bool, error) {
		val, ok := s.env[arg]
		s.context.Log(val)
		return ok, nil
	},
	"fatal-internal-error": func(s *sandbox, arg string) (bool, error) {
		// Should normally only be used if error is reported with Monitor
		return false, runtime.ErrFatalInternalError
	},
	"nonfatal-internal-error": func(s *sandbox, arg string) (bool, error) {
		// Should normally only be used if error is reported with Monitor
		return false, runtime.ErrNonFatalInternalError
	},
	"malformed-payload-after-start": func(s *sandbox, arg string) (bool, error) {
		return false, runtime.NewMalformedPayloadError(s.payload.Argument)
	},
	"stopNow-sleep": func(s *sandbox, arg string) (bool, error) {
		// This is not really a reasonable thing for an engine to do. But it's
		// useful for testing... StopNow causes all running tasks to be resolved
		// 'exception' with reason: 'worker-shutdown'.
		s.environment.Worker.StopNow()
		time.Sleep(500 * time.Millisecond)
		return true, nil
	},
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	s.resolve.Wait()
	if s.resultErr != nil {
		return nil, s.resultErr
	}
	return s, nil
}

func (s *sandbox) Kill() error {
	s.resolve.Do(func() {
		s.abortSessions()
		s.result = false
		s.abortErr = engines.ErrSandboxTerminated
	})
	s.resolve.Wait()
	return s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		s.abortSessions()
		s.result = false
		s.resultErr = engines.ErrSandboxAborted
	})
	s.resolve.Wait()
	return s.abortErr
}

func (s *sandbox) NewShell(command []string, tty bool) (engines.Shell, error) {
	s.Lock()
	defer s.Unlock()

	if len(command) > 0 || tty {
		return nil, engines.ErrFeatureNotSupported
	}
	if s.sessions.Add(1) != nil {
		return nil, engines.ErrSandboxTerminated
	}
	shell := newShell()
	s.shells = append(s.shells, shell)
	go func() {
		shell.Wait()
		s.sessions.Done()
	}()
	return shell, nil
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
	s.Lock()
	defer s.Unlock()

	if name != "MockDisplay" {
		return nil, engines.ErrNoSuchDisplay
	}
	if s.sessions.Add(1) != nil {
		return nil, engines.ErrSandboxTerminated
	}
	d := ioext.WatchPipe(newMockDisplay(), func(error) {
		s.sessions.Done()
	})
	s.displays = append(s.displays, d)
	return d, nil
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
	for p, data := range s.files {
		if strings.HasPrefix(p, folder) {
			foundFolder = true
			wg.Add(1)
			go func(p string, data []byte) {
				p = p[len(folder):] // Note: folder always ends with slash
				err := handler(p, ioext.NopCloser(bytes.NewReader(data)))
				if err != nil {
					m.Lock()
					handlerError = true
					m.Unlock()
				}
				wg.Done()
			}(p, data)
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
