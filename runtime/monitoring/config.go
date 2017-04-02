package monitoring

import (
	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

var mockConfigSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"type": schematypes.StringEnum{Options: []string{"mock"}},
		"panicOnError": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title:       "Panic On Error",
				Description: "Use a mock implementation of the monitor that panics on errors.",
			},
		},
	},
	Required: []string{"type", "panicOnError"},
}

var monitorConfigSchema schematypes.Schema = schematypes.Object{
	Properties: schematypes.Properties{
		"project": schematypes.String{
			MetaData: schematypes.MetaData{
				Title:       "Sentry/Statsum Project Name",
				Description: "Project name to be used in sentry and statsum",
			},
			Pattern: "^[a-zA-Z0-9_-]{1,22}$",
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
			MetaData: schematypes.MetaData{
				Title:       "Tags",
				Description: "Tags that should be applied to all logs/sentry entries from this worker",
			},
			Values: schematypes.String{},
		},
	},
	Required: []string{"logLevel"},
}

// ConfigSchema for configuration given to New()
var ConfigSchema schematypes.Schema = schematypes.OneOf{
	mockConfigSchema,
	monitorConfigSchema,
}

// New returns a runtime.Monitor strategy from config matching ConfigSchema.
func New(config interface{}, auth client.Auth) runtime.Monitor {
	schematypes.MustValidate(ConfigSchema, config)

	// try monitor schema
	var c struct {
		Project  string            `json:"project"`
		LogLevel string            `json:"logLevel"`
		Tags     map[string]string `json:"tags"`
	}
	if schematypes.MustMap(monitorConfigSchema, config, &c) == nil {
		if c.Project != "" {
			return NewMonitor(c.Project, auth, c.LogLevel, c.Tags)
		}
		return NewLoggingMonitor(c.LogLevel, c.Tags)
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
