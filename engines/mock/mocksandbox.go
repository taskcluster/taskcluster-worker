package mockengine

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
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
	mounts  map[string]*mount
	proxies map[string]http.Handler
	result  bool
}

///////////////////////////// Implementation of SandboxBuilder interface

func (s *sandbox) StartSandbox() (engines.Sandbox, error) {
	s.Lock()
	defer s.Unlock()
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
	if s.mounts[mountpoint] != nil {
		return engines.NewMalformedPayloadError("mountpoint is conflicting!")
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
	if s.proxies[name] != nil {
		return engines.NewMalformedPayloadError("proxy name is conflicting!")
	}
	s.proxies[name] = handler
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
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	// No need to lock access to payload, as it can't be mutated at this point
	time.Sleep(time.Duration(s.payload.Delay) * time.Millisecond)
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

///////////////////////////// Implementation of ResultSet interface

func (s *sandbox) Success() bool {
	// No need to lock access as result is immutable
	return s.result
}
