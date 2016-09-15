package interactive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

// defaultArtifactPrefix is the default artifact prefix used if nothing is
// configured or given in the task definition
const defaultArtifactPrefix = "private/interactive/"

// defaultShellToolURL is the default URL for the tool that can connect to the
// shell socket and display an interactive shell session.
const defaultShellToolURL = "https://tools.taskcluster.net/shell/"

// defaultShellToolURL is the default URL for the tool that can list displays
// and connect to the display socket with interactive noVNC session.
const defaultDisplayToolURL = "https://tools.taskcluster.net/display/"

type provider struct {
	plugins.PluginProviderBase
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}
func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	var c config
	if schematypes.MustMap(configSchema, options.Config, &c) != nil {
		return nil, engines.ErrContractViolation
	}
	if c.ArtifactPrefix == "" {
		c.ArtifactPrefix = defaultArtifactPrefix
	}
	if c.ShellToolURL == "" {
		c.ShellToolURL = defaultShellToolURL
	}
	if c.DisplayToolURL == "" {
		c.DisplayToolURL = defaultDisplayToolURL
	}
	return &plugin{
		config: c,
		log:    options.Log,
	}, nil
}

type plugin struct {
	plugins.PluginBase
	config config
	log    *logrus.Entry
}

func (p *plugin) PayloadSchema() schematypes.Object {
	s := schematypes.Object{
		MetaData: schematypes.MetaData{
			Title: "Interactive Features",
			Description: `Settings for interactive features, all options are optional
				an empty object can be used to enable the interactive features with
				default options.`,
		},
		Properties: schematypes.Properties{
			"disableDisplay": schematypes.Boolean{},
			"disableShell":   schematypes.Boolean{},
		},
	}
	if !p.config.ForbidCustomArtifactPrefix {
		s.Properties["artifactPrefix"] = schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "Artifact Prefix",
				Description: "Prefix for the interactive artifacts will be used to " +
					"create `<prefix>/shell.html`, `<prefix>/display.html` and " +
					"`<prefix>/sockets.json`. The prefix defaults to `" +
					p.config.ArtifactPrefix + "`",
			},
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
	if schematypes.MustMap(p.PayloadSchema(), options.Payload, &P) != nil {
		return nil, engines.ErrContractViolation
	}
	// If not always enabled or no options are given then this is disabled
	if P.Interactive == nil && !p.config.AlwaysEnabled {
		return nil, nil
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
		opts:   o,
		log:    options.Log,
		parent: p,
	}, nil
}

type taskPlugin struct {
	plugins.TaskPluginBase
	parent           *plugin
	log              *logrus.Entry
	opts             opts
	sandbox          engines.Sandbox
	context          *runtime.TaskContext
	shellURL         string
	shellServer      *ShellServer
	displaysURL      string
	displaySocketURL string
	displayServer    *displayServer
}

func (p *taskPlugin) Prepare(context *runtime.TaskContext) error {
	p.context = context
	return nil
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
	if p.shellServer != nil {
		p.shellServer.Abort()
	}
	if p.displayServer != nil {
		p.displayServer.Abort()
	}
	return true, nil
}

func (p *taskPlugin) Dispose() error {
	if p.shellServer != nil {
		p.shellServer.Abort()
		p.shellServer.Wait()
	}
	if p.displayServer != nil {
		p.displayServer.Abort()
	}
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
		p.sandbox.NewShell, p.log.WithField("interactive", "shell"),
	)
	u := p.context.AttachWebHook(p.shellServer)
	p.shellURL = urlProtocolToWebsocket(u)

	query := url.Values{}
	query.Set("v", "2")
	query.Set("taskId", p.context.TaskID)
	query.Set("runId", fmt.Sprintf("%d", p.context.RunID))
	query.Set("socketUrl", p.shellURL)

	return runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     p.opts.ArtifactPrefix + "shell.html",
		Mimetype: "text/html",
		URL:      p.parent.config.ShellToolURL + "?" + query.Encode(),
		Expires:  p.context.Deadline,
	}, p.context)
}

func (p *taskPlugin) setupDisplay() error {
	// Setup display if not disabled
	if p.opts.DisableDisplay {
		return nil
	}
	debug("Setting up interactive display")

	// Create display server
	p.displayServer = newDisplayServer(
		p.sandbox, p.log.WithField("interactive", "display"),
	)
	u := p.context.AttachWebHook(p.displayServer)
	p.displaysURL = u
	p.displaySocketURL = urlProtocolToWebsocket(u)

	query := url.Values{}
	query.Set("v", "1")
	query.Set("taskId", p.context.TaskID)
	query.Set("runId", fmt.Sprintf("%d", p.context.RunID))
	query.Set("socketUrl", p.displaySocketURL)
	query.Set("displaysUrl", p.displaysURL)

	return runtime.CreateRedirectArtifact(runtime.RedirectArtifact{
		Name:     p.opts.ArtifactPrefix + "display.html",
		Mimetype: "text/html",
		URL:      p.parent.config.DisplayToolURL + "?" + query.Encode(),
		Expires:  p.context.Deadline,
	}, p.context)
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
	return runtime.UploadS3Artifact(runtime.S3Artifact{
		Name:     p.opts.ArtifactPrefix + "sockets.json",
		Mimetype: "application/json",
		Expires:  p.context.Deadline,
		Stream:   ioext.NopCloser(bytes.NewReader(data)),
	}, p.context)
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
