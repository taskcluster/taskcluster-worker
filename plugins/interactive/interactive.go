package interactive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

// defaultArtifactPrefix is the default artifact prefix used if nothing is
// configured or given in the task definition
const defaultArtifactPrefix = "private/interactive/"

type provider struct {
	plugins.PluginProviderBase
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}
func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	schematypes.MustValidateAndMap(configSchema, options.Config, &c)
	toolsURL := options.Environment.GetServiceURL("tools")

	if c.ArtifactPrefix == "" {
		c.ArtifactPrefix = defaultArtifactPrefix
	}
	if c.ShellToolURL == "" {
		c.ShellToolURL = fmt.Sprintf("%s/shell", toolsURL)
	}
	if c.DisplayToolURL == "" {
		c.DisplayToolURL = fmt.Sprintf("%s/display", toolsURL)
	}

	// IF no WebHookServer is available we disabling the interactive plugin
	if options.Environment.WebHookServer == nil {
		options.Monitor.Info("interactive plugin should be disabled when no WebHookServer is configured")
		return plugins.PluginBase{}, nil
	}

	return &plugin{
		config:        c,
		monitor:       options.Monitor,
		webhookserver: options.Environment.WebHookServer,
	}, nil
}

type plugin struct {
	plugins.PluginBase
	config        config
	monitor       runtime.Monitor
	webhookserver webhookserver.WebHookServer
}

func (p *plugin) PayloadSchema() schematypes.Object {
	s := schematypes.Object{
		Title: "Interactive Features",
		Description: util.Markdown(`
			Settings for interactive features, all options are optional,
			an empty object can be used to enable the interactive features with
			default options.
		`),
		Properties: schematypes.Properties{
			"disableDisplay": schematypes.Boolean{
				Title: "Disable Display",
				Description: util.Markdown(`
					Disable the interactive display, defaults to enabled if any options
					is given for 'interactive', even an empty object.
				`),
			},
			"disableShell": schematypes.Boolean{
				Title: "Disable Shell",
				Description: util.Markdown(`
					Disable the interactive shell, defaults to enabled if any options
					is given for 'interactive', even an empty object.
				`),
			},
		},
	}
	if !p.config.ForbidCustomArtifactPrefix {
		s.Properties["artifactPrefix"] = schematypes.String{
			Title: "Artifact Prefix",
			Description: util.Markdown(`
				Prefix for the interactive artifacts will be used to create
				'<prefix>/shell.html', '<prefix>/display.html' and
				'<prefix>/sockets.json'. The prefix defaults to
				'` + p.config.ArtifactPrefix + `'.
			`),
			Pattern:       `^[\x20-.0-\x7e][\x20-\x7e]*/$`,
			MaximumLength: 255,
		}
	}
	return schematypes.Object{
		Properties: schematypes.Properties{
			"interactive": s,
		},
	}
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (
	plugins.TaskPlugin, error,
) {
	var P payload
	schematypes.MustValidateAndMap(p.PayloadSchema(), options.Payload, &P)

	// If not always enabled or no options are given then this is disabled
	if P.Interactive == nil && !p.config.AlwaysEnabled {
		return plugins.TaskPluginBase{}, nil
	}

	// Extract options
	o := opts{}
	if P.Interactive != nil {
		o = *P.Interactive
	}
	if o.ArtifactPrefix == "" || p.config.ForbidCustomArtifactPrefix {
		o.ArtifactPrefix = p.config.ArtifactPrefix
	}

	return &taskPlugin{
		context:  options.TaskContext,
		webhooks: webhookserver.NewWebHookSet(p.webhookserver),
		opts:     o,
		monitor:  options.Monitor,
		parent:   p,
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	parent           *plugin
	webhooks         *webhookserver.WebHookSet
	monitor          runtime.Monitor
	opts             opts
	sandbox          engines.Sandbox
	context          *runtime.TaskContext
	shellURL         string
	shellServer      *ShellServer
	displaysURL      string
	displaySocketURL string
	displayServer    *DisplayServer
}

func (p *taskPlugin) Started(sandbox engines.Sandbox) error {
	p.sandbox = sandbox

	// Setup shell and display in parallel
	wg := sync.WaitGroup{}
	wg.Add(2)
	var err1, err2 error
	go func() {
		err1 = p.setupShell()
		wg.Done()
	}()
	go func() {
		err2 = p.setupDisplay()
		wg.Done()
	}()
	wg.Wait()

	// Return any of the two errors
	if err1 != nil {
		return fmt.Errorf("Setting up interactive shell failed, error: %s", err1)
	}
	if err2 != nil {
		return fmt.Errorf("Setting up interactive display failed, error: %s", err2)
	}

	err := p.createSocketsFile()
	if err != nil {
		return fmt.Errorf("Failed to create sockets.json file, error: %s", err)
	}
	return nil
}

func (p *taskPlugin) Stopped(_ engines.ResultSet) (bool, error) {
	return true, p.Dispose()
}

func (p *taskPlugin) Exception(_ runtime.ExceptionReason) error {
	return p.Dispose()
}

func (p *taskPlugin) Dispose() error {
	// NOTE: This is also called from Stopped() and Exception()
	util.Parallel(func() {
		if p.shellServer != nil {
			p.shellServer.Abort()
			p.shellServer.Wait()
		}
		p.shellServer = nil
	}, func() {
		if p.displayServer != nil {
			p.displayServer.Abort()
		}
		p.displayServer = nil
	}, func() {
		if p.webhooks != nil {
			p.webhooks.Dispose()
		}
		p.webhooks = nil
	})
	return nil
}

func (p *taskPlugin) setupShell() error {
	// Setup shell if not disabled
	if p.opts.DisableShell {
		return nil
	}
	debug("Setting up interactive shell")

	// Create shell server and get a URL to reach it
	p.shellServer = NewShellServer(
		p.sandbox.NewShell, p.monitor.WithPrefix("shell-server"),
	)
	u := p.webhooks.AttachHook(p.shellServer)
	p.shellURL = urlProtocolToWebsocket(u)

	query := url.Values{}
	query.Set("v", "2")
	query.Set("taskId", p.context.TaskID)
	query.Set("runId", fmt.Sprintf("%d", p.context.RunID))
	query.Set("socketUrl", p.shellURL)

	return p.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     p.opts.ArtifactPrefix + "shell.html",
		Mimetype: "text/html",
		URL:      p.parent.config.ShellToolURL + "?" + query.Encode(),
		Expires:  p.context.TaskInfo.Deadline,
	})
}

func (p *taskPlugin) setupDisplay() error {
	// Setup display if not disabled
	if p.opts.DisableDisplay {
		return nil
	}
	debug("Setting up interactive display")

	// Create display server
	p.displayServer = NewDisplayServer(
		p.sandbox, p.monitor.WithPrefix("display-server"),
	)
	u := p.webhooks.AttachHook(p.displayServer)
	p.displaysURL = u
	p.displaySocketURL = urlProtocolToWebsocket(u)

	query := url.Values{}
	query.Set("v", "1")
	query.Set("taskId", p.context.TaskID)
	query.Set("runId", fmt.Sprintf("%d", p.context.RunID))
	query.Set("socketUrl", p.displaySocketURL)
	query.Set("displaysUrl", p.displaysURL)
	// TODO: Make this an option the engine can specify in ListDisplays
	//       Probably requires changing display list result to contain websocket
	//       URLs. Hence, introducing v=2, so leaving it for later.
	query.Set("shared", "true")

	return p.context.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     p.opts.ArtifactPrefix + "display.html",
		Mimetype: "text/html",
		URL:      p.parent.config.DisplayToolURL + "?" + query.Encode(),
		Expires:  p.context.TaskInfo.Deadline,
	})
}

func (p *taskPlugin) createSocketsFile() error {
	debug("Uploading sockets.json")
	// Create sockets.json
	sockets := map[string]interface{}{
		"version": 2,
	}
	if p.shellURL != "" {
		sockets["shellSocketUrl"] = p.shellURL
	}
	if p.displaysURL != "" {
		sockets["displaysUrl"] = p.displaysURL
	}
	if p.displaySocketURL != "" {
		sockets["displaySocketUrl"] = p.displaySocketURL
	}
	data, _ := json.MarshalIndent(sockets, "", "  ")
	return p.context.UploadS3Artifact(runtime.S3Artifact{
		Name:     p.opts.ArtifactPrefix + "sockets.json",
		Mimetype: "application/json",
		Expires:  p.context.TaskInfo.Deadline,
		Stream:   ioext.NopCloser(bytes.NewReader(data)),
	})
}

func urlProtocolToWebsocket(u string) string {
	if strings.HasPrefix(u, "http://") {
		return "ws://" + u[7:]
	}
	if strings.HasPrefix(u, "https://") {
		return "wss://" + u[8:]
	}
	return u
}
