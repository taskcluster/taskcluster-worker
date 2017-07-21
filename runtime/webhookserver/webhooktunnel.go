package webhookserver

import (
	"errors"
	"net/http"
	"sync"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/auth"
	"github.com/taskcluster/webhooktunnel/util"
	"github.com/taskcluster/webhooktunnel/whclient"
)

// WebhookTunnel
type WebhookTunnel struct {
	m        sync.RWMutex
	handlers map[string]http.Handler
	// url is assigned when the client connects to the proxy
	client *whclient.Client
}

// NewWebhookTunnel returns a pointer to a new WebhookTunnel instance
func NewWebhookTunnel(credentials *tcclient.Credentials) (*WebhookTunnel, error) {
	configurer := func() (whclient.Config, error) {
		authClient := auth.New(credentials)
		whresp, err := authClient.WebhooktunnelToken()
		if err != nil {
			return whclient.Config{}, errors.New("could not get token from tc-auth")
		}

		// This hack is needed since proxyURL in auth config is set as a http url
		proxyURL := whresp.ProxyURL
		if proxyURL[:2] != "ws" {
			proxyURL = util.MakeWsURL(proxyURL)
		}

		return whclient.Config{
			ID:        whresp.TunnelID,
			ProxyAddr: proxyURL,
			Token:     whresp.Token,
		}, nil
	}

	client, err := whclient.New(configurer)
	if err == whclient.ErrAuthFailed {
		client, err = whclient.New(configurer)
	}

	if err != nil {
		return nil, err
	}

	wt := &WebhookTunnel{
		handlers: make(map[string]http.Handler),
		client:   client,
	}

	go func() {
		http.Serve(wt.client, http.HandlerFunc(wt.handle))
	}()
	return wt, nil
}

// AttachHook adds a new webhook to the server
func (wt *WebhookTunnel) AttachHook(handler http.Handler) (string, func()) {
	id := slugid.Nice()
	wt.m.Lock()
	wt.handlers[id] = handler
	wt.m.Unlock()

	url := wt.client.URL() + "/" + id + "/"
	detach := func() {
		wt.m.Lock()
		defer wt.m.Unlock()
		delete(wt.handlers, id)
	}

	return url, detach
}

// Stop will close the webhooktunnel client
func (wt *WebhookTunnel) Stop() {
	_ = wt.client.Close()
}

func (wt *WebhookTunnel) handle(w http.ResponseWriter, r *http.Request) {
	id, path := r.URL.Path[1:23], r.URL.Path[23:]

	wt.m.RLock()
	handler, ok := wt.handlers[id]
	wt.m.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	r.URL.Path = path
	handler.ServeHTTP(w, r)
}
