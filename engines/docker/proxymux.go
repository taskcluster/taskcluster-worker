// +build linux

package dockerengine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type proxyErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (p proxyErrorPayload) MustMarshalJSON() []byte {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		panic(errors.Wrap(err, "json.MarshalIndent on proxyErrorPayload"))
	}
	return data
}

// proxyMux takes requests where the path is /<name>/<path> then forwards it to
// Proxies[name] with <path> as if <path> had been hit.
type proxyMux struct {
	Proxies     map[string]http.Handler
	TaskContext *runtime.TaskContext
}

func (p *proxyMux) writeInvalidServiceRequest(w http.ResponseWriter, originalPath string) {
	// Find active services in this container
	services := []string{}
	for key := range p.Proxies {
		services = append(services, key)
	}
	servicesJSON, err := json.Marshal(services)
	if err != nil {
		panic(errors.Wrap(err, "json.Marshal failed to render list of services"))
	}

	// Print message to task log, this makes debugging tasks a lot easier
	p.TaskContext.LogError(fmt.Sprintf("http://taskcluster/%s is not a legal service path, "+
		"must be on the form `http://taskcluster/<name>/<path>`; This container has the following services: %s",
		originalPath, string(servicesJSON)))

	// Write 404 response
	w.WriteHeader(http.StatusNotFound)
	w.Write(proxyErrorPayload{
		Code: "InvalidWorkerRequestError",
		Message: "Requests to `http://taskcluster` must be on the form `http://taskcluster/<name>/<path>` " +
			"where `<name>` is a service you want to hit. This container has the following services: " +
			string(servicesJSON),
	}.MustMarshalJSON())
}

func (p *proxyMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Sanity checks and identifiation of name/hostname/virtualhost/folder
	var origPath string
	isRawPath := r.URL.RawPath != ""
	if isRawPath {
		origPath = r.URL.RawPath
	} else {
		origPath = r.URL.Path
	}
	debug("handling request: %s", origPath)

	if len(origPath) == 0 || origPath[0] != '/' {
		p.writeInvalidServiceRequest(w, origPath)
		return
	}
	parts := strings.SplitN(origPath[1:], "/", 2)
	if len(parts) != 2 {
		p.writeInvalidServiceRequest(w, origPath)
		return
	}
	name, path := parts[0], "/"+parts[1]

	// Find the handler in for given name
	h := p.Proxies[name]
	if h == nil {
		p.writeInvalidServiceRequest(w, origPath)
		return
	}

	// Rewrite the path
	if isRawPath {
		r.URL.Path, _ = url.PathUnescape(path)
		r.URL.RawPath = path
	} else {
		r.URL.Path = path
		r.URL.RawPath = ""
	}

	h.ServeHTTP(w, r)
}
