package relengapi

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/pkg/errors"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

const tmpTokenLifetime time.Duration = 10 * time.Minute
const tmpTokenSkew time.Duration = 10 * time.Second

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	config config
}

type taskPlugin struct {
	plugins.TaskPluginBase
	monitor           runtime.Monitor
	proxy             httputil.ReverseProxy
	context           *runtime.TaskContext
	tmpToken          string
	tmpTokenGoodUntil time.Time
	permissions       []string
	relengapiDomain   string
}

func init() {
	plugins.Register("relengapi", provider{})
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (p *plugin) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)
	if c.Domain == "" {
		c.Domain = "mozilla-releng.net"
	}

	if c.Token == "" {
		err := errors.New("Token value not configured for relengapi plugin")
		debug(fmt.Sprintln(err))
		options.Monitor.Error(err)
		return nil, err
	}

	return &plugin{
		PluginBase: plugins.PluginBase{},
		config:     c,
	}, nil
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	var P payload
	schematypes.MustValidateAndMap(payloadSchema, options.Payload, &P)

	// If disabled, we return nothing
	if !P.EnableRelengAPIProxy {
		return plugins.TaskPluginBase{}, nil
	}

	debug(fmt.Sprintf("Starting relengapi plugin for task %s", options.TaskContext.TaskID))

	allPerms := []string{
		"base.tokens.tmp.issue",
		"base.tokens.usr.issue",
		"base.tokens.usr.revoke.my",
		"base.tokens.usr.view.my",
		"clobberer.post.clobber",
		"tooltool.download.internal",
		"tooltool.download.public",
		"tooltool.upload.internal",
		"tooltool.upload.public",
	}

	perms := []string{}
	for _, p := range allPerms {
		if options.TaskContext.HasScopes([]string{"worker:relengapi-proxy:" + p}) {
			perms = append(perms, p)
		}
	}

	tp := &taskPlugin{
		monitor:         options.Monitor,
		context:         options.TaskContext,
		permissions:     perms,
		relengapiDomain: p.config.Domain,
	}

	// stolen from relengapi-proxy
	director := func(req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/tooltool") {
			req.URL.Scheme = "https"
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/tooltool")
			req.URL.RawPath = ""
			host := fmt.Sprintf("tooltool.%s", p.config.Domain)
			req.URL.Host = host
			req.Host = host
		} else if strings.HasPrefix(req.URL.Path, "/treestatus") {
			req.URL.Scheme = "https"
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/treestatus")
			req.URL.RawPath = ""
			host := fmt.Sprintf("treestatus.%s", p.config.Domain)
			req.URL.Host = host
			req.Host = host
		} else if strings.HasPrefix(req.URL.Path, "/mapper") {
			req.URL.Scheme = "https"
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/mapper")
			req.URL.RawPath = ""
			host := fmt.Sprintf("mapper.%s", p.config.Domain)
			req.URL.Host = host
			req.Host = host
		} else {
			// ignore everything else
			return
		}
		tok, err := tp.getToken(p.config.Token)
		if err != nil {
			err = errors.Wrap(err, "Error retrieving token")
			debug(fmt.Sprintln(err))
			options.TaskContext.Log(err)
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tok))
		}
		options.TaskContext.Log(fmt.Sprint(req.Method, req.URL))
	}

	tp.proxy = httputil.ReverseProxy{
		Director: director,
	}

	return tp, nil
}

func (p *taskPlugin) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	err := sandboxBuilder.AttachProxy("relengapi", p)
	if err == engines.ErrFeatureNotSupported {
		// Fire off a warning, and then do nothing...
		p.monitor.ReportWarning(err, "plugin 'relengapi' is enabled, but the engine doesn't support proxy attachments")
		return nil
	}
	if err == engines.ErrNamingConflict {
		err = runtime.NewMalformedPayloadError("the proxy name 'relengapi' is already in use")
		debug(err.Error())
		return err
	}
	if _, ok := runtime.IsMalformedPayloadError(err); ok {
		// the name "relengapi" is not allowed by the engine, we assume it to be safe,
		// so if it's not we'll panic
		panic(errors.Wrap(err, "proxy name 'relengapi' is not permitted by the engine"))
	}
	return nil
}

func (p *taskPlugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

func (p *taskPlugin) getToken(issuingToken string) (string, error) {
	now := time.Now()
	if now.After(p.tmpTokenGoodUntil) {
		expires := now.Add(tmpTokenLifetime)
		debug("Generating new temporary token; expires at " + expires.String())
		url := fmt.Sprintf("https://tokens.%s", p.relengapiDomain)
		tok, err := getTmpToken(url, issuingToken, expires, p.permissions)
		if err != nil {
			return "", err
		}
		p.tmpToken = tok
		p.tmpTokenGoodUntil = expires.Add(-tmpTokenSkew)
	}
	return p.tmpToken, nil
}
