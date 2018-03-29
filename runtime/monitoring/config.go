package monitoring

import (
	"github.com/sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

var mockConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"type": schematypes.StringEnum{Options: []string{"mock"}},
		"panicOnError": schematypes.Boolean{
			Title:       "Panic On Error",
			Description: "Use a mock implementation of the monitor that panics on errors.",
		},
	},
	Required: []string{"type", "panicOnError"},
}

var monitorConfigSchema schematypes.Schema = schematypes.Object{
	Properties: schematypes.Properties{
		"project": schematypes.String{
			Title:       "Sentry/Statsum Project Name",
			Description: "Project name to be used in sentry and statsum",
			Pattern:     "^[a-zA-Z0-9_-]{1,22}$",
		},
		"logLevel": schematypes.StringEnum{
			Options: []string{
				logrus.DebugLevel.String(),
				logrus.InfoLevel.String(),
				logrus.WarnLevel.String(),
				logrus.ErrorLevel.String(),
				logrus.FatalLevel.String(),
				logrus.PanicLevel.String(),
			},
		},
		"tags": schematypes.Map{
			Title:       "Tags",
			Description: "Tags that should be applied to all logs/sentry entries from this worker",
			Values:      schematypes.String{},
		},
		"syslog": schematypes.String{
			Title:       "Syslog Name",
			Description: "Name to use for process in syslog, leave as empty string to disable syslog forwarding.",
		},
	},
	Required: []string{"logLevel"},
}

// ConfigSchema for configuration given to New()
var ConfigSchema schematypes.Schema = schematypes.OneOf{
	mockConfigSchema,
	monitorConfigSchema,
}

// PreConfig returns a default monitor for use before the configuration is loaded.  This logs at
// the INFO level to stderr.
func PreConfig() runtime.Monitor {
	return NewLoggingMonitor("info", map[string]string{}, "taskcluster-worker")
}

// New returns a runtime.Monitor strategy from config matching ConfigSchema.
func New(config interface{}, auth client.Auth) runtime.Monitor {
	schematypes.MustValidate(ConfigSchema, config)

	// try monitor schema
	var c struct {
		Project  string            `json:"project"`
		LogLevel string            `json:"logLevel"`
		Tags     map[string]string `json:"tags"`
		Syslog   string            `json:"syslog"`
	}
	if schematypes.MustMap(monitorConfigSchema, config, &c) == nil {
		if c.Project != "" {
			return NewMonitor(c.Project, auth, c.LogLevel, c.Tags, c.Syslog)
		}
		return NewLoggingMonitor(c.LogLevel, c.Tags, c.Syslog)
	}

	// try mock schema
	var m struct {
		Type         string `json:"type"`
		PanicOnError bool   `json:"panicOnError"`
	}
	if schematypes.MustMap(mockConfigSchema, config, &m) == nil {
		return mocks.NewMockMonitor(m.PanicOnError)
	}

	panic("monitor should have matched one of the options, this should be impossible")
}
