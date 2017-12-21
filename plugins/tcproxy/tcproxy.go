package tcproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
}

type taskPlugin struct {
	plugins.TaskPluginBase
	monitor runtime.Monitor
	context *runtime.TaskContext
}

func init() {
	plugins.Register("tcproxy", provider{})
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	return &plugin{}, nil
}

func (p *plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P payload
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &P)

	// If disabled we return nothing
	if P.DisableTaskclusterProxy {
		return plugins.TaskPluginBase{}, nil
	}

	return &taskPlugin{
		monitor: options.Monitor,
		context: options.TaskContext,
	}, nil
}

func (p *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	err := sandboxBuilder.AttachProxy("tcproxy", p)
	if err == engines.ErrFeatureNotSupported {
		// Fire off a warning, and then do nothing...
		p.monitor.ReportWarning(err, "plugin 'tcproxy' is enabled, but the engine doesn't support proxy attachments")
		return nil
	}
	if err == engines.ErrNamingConflict {
		return runtime.NewMalformedPayloadError("the proxy name 'tcproxy' is already in use")
	}
	if _, ok := runtime.IsMalformedPayloadError(err); ok {
		// the name "tcproxy" is not allowed by the engine, we assume it to be safe,
		// so if it's not we'll panic
		panic(errors.Wrap(err, "proxy name 'tcproxy' is not permitted by the engine"))
	}
	return nil
}

func (p *taskPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse request URL
	raw := strings.TrimPrefix(r.URL.Path, "/")
	u, err := url.Parse("https://" + raw)
	if err != nil {
		debug("bad URL: '%s'", r.URL.Path)
		p.context.LogError(fmt.Sprintf("tcproxy received path: '%s' which it failed to parse as a URL", raw))
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.MarshalIndent(struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "InvalidRequestUrl",
			Message: "tcproxy assumes <path> is on the form <hostname>/<resource>[?<query>], instead received: '" + raw + "'",
		}, "", "  ")
		w.Write(data)
		return
	}

	debug("proxing: '%s'", raw)

	// Create request
	req, _ := http.NewRequest(r.Method, u.String(), r.Body)

	// Add headers except 'Host'
	r.Header.Del("Host")
	for k, v := range r.Header {
		req.Header[k] = v
	}

	// Add signature
	signature, err := p.context.Authorizer().SignHeader(r.Method, u, nil)
	if err != nil {
		incidentID := p.monitor.ReportError(
			errors.Wrap(err, "SignHeader failed"),
			"SignHeader failed for URL: ", u.String(),
		)
		p.context.LogError(fmt.Sprintf("tcproxy expirenced an internal error, incidentID: %s", incidentID))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{
      "code": "InternalServerError",
      "message": "internal error in taskcluster-worker proxy, incidentId: ` + incidentID + `"
    }`))
		return
	}
	req.Header.Set("Authorization", signature)

	// Set TaskContext, so request can be aborted
	req = req.WithContext(p.context)

	// Send request
	res, err := http.DefaultClient.Do(req)

	// Handle task aborting
	if p.context.Err() != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{
      "code": "TaskResolved",
      "message": "task was resolved, aborting proxied requests"
    }`))
		// Quick clean-up body, if for some reason cancel happened after response
		if err == nil && res.Body != nil {
			res.Body.Close()
		}
		return
	}

	// Handle connection errors
	if err != nil {
		// This could be task that canceled the request, or task connection broke
		// and r.Body returned an error or something like that... Or it could be
		// the request URL was bad, or service was down...
		p.context.Log(fmt.Sprintf(
			"tcproxy was unable to forward request to %s, error: %s",
			u.String(), err,
		))
		w.WriteHeader(http.StatusInternalServerError)
		data, _ := json.MarshalIndent(struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "ProxyRequestFailed",
			Message: "tcproxy failed forwarding request to '" + u.String() + "'",
		}, "", "  ")
		w.Write(data)
		return
	}

	// Set headers from res
	for k, v := range res.Header {
		w.Header()[k] = v
	}

	// Write header
	w.WriteHeader(res.StatusCode)

	// Copy body
	_, err = io.Copy(w, res.Body)
	if err != nil {
		debug("failed for proxy response, error: %s", err)
		p.context.Log(fmt.Sprintf("tcproxy failed to proxy the entire response from: %s", u.String()))
	}
}
