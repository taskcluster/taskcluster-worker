package webhookserver

import "net/http"

// noneServer is a stupid implementation that does nothing.
type noneServer struct{}

func (noneServer) AttachHook(handler http.Handler) (url string, detach func()) {
	return "", func() {}
}

func (noneServer) Stop() {

}
