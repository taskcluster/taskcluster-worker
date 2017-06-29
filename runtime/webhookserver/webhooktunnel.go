package webhookserver

import (
	"net/http"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/webhooktunnel/whclient"
)

func genWebhooktunnelAuthorizer(secret string) whclient.Authorizer {
	return func(id string) (string, error) {
		now := time.Now()
		expires := now.Add(30 * 24 * time.Hour)
		token := jwt.New(jwt.SigningMethodHS256)
		token.Claims.(jwt.MapClaims)["iat"] = now.Unix() - 300
		token.Claims.(jwt.MapClaims)["exp"] = expires.Unix()
		token.Claims.(jwt.MapClaims)["nbf"] = now.Unix() - 300

		token.Claims.(jwt.MapClaims)["tid"] = id
		tokenString, err := token.SignedString([]byte(secret))
		return tokenString, err
	}
}

// WebhookTunnel
type WebhookTunnel struct {
	m        sync.RWMutex
	handlers map[string]http.Handler
	// url is assigned when the client connects to the proxy
	client *whclient.Client
}

// NewWebhookTunnel returns a pointer to a new WebhookTunnel instance
func NewWebhookTunnel(workerID string, proxyAddr string, authorizer whclient.Authorizer) (*WebhookTunnel, error) {
	client, err := whclient.New(whclient.Config{
		ID:        workerID,
		ProxyAddr: proxyAddr,
		Authorize: authorizer,
	})

	if err != nil {
		return nil, err
	}

	wt := &WebhookTunnel{
		handlers: make(map[string]http.Handler),
		client:   client,
	}

	go http.Serve(wt.client, http.HandlerFunc(wt.handle))
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
