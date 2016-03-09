package webhookserver

import (
	"net/http"
	"sync"
)

// A WebHookSet wraps a WebHookServer such that all hooks can be detached when
// the WebHookSet is disposed. This is useful for scoping a hook to a
// task-cycle.
type WebHookSet struct {
	m           sync.Mutex
	server      WebHookServer
	detachFuncs []func()
}

// NewWebHookSet returns a new WebHookSet wrapping the given WebHookServer
func NewWebHookSet(server WebHookServer) *WebHookSet {
	return &WebHookSet{server: server}
}

// AttachHook returns a url-prefix for which requests will be given to handler.
func (s *WebHookSet) AttachHook(handler http.Handler) (url string) {
	s.m.Lock()
	defer s.m.Unlock()

	url, detach := s.server.AttachHook(handler)
	s.detachFuncs = append(s.detachFuncs, detach)
	return
}

// Dispose clears all hooks attached through this WebHookSet
func (s *WebHookSet) Dispose() {
	s.m.Lock()
	defer s.m.Unlock()

	for _, detach := range s.detachFuncs {
		detach()
	}
	s.detachFuncs = nil
}
