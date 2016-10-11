// +build darwin

package osxnative

import (
	"sync"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
)

type engine struct {
	engines.EngineBase
	config *configType
	log    *logrus.Entry
}

type engineProvider struct {
	engines.EngineProviderBase
}

func (e engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	var c configType
	if err := schematypes.MustMap(configSchema, options.Config, &c); err != nil {
		options.Log.WithError(err).Error("Invalid configuration")
		return nil, engines.ErrContractViolation
	}

	return &engine{
		config: &c,
		log:    options.Log,
	}, nil
}

type payloadType struct {
	Link    string   `json:"link"`
	Command []string `json:"command"`
}

type configType struct {
	CreateUser bool     `json:"createUser"`
	UserGroups []string `json:"userGroups"`
	Sudo       bool     `json:"sudo"`
}

var payloadSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"link": schematypes.URI{
			MetaData: schematypes.MetaData{
				Title: "Executable to download",
				Description: `Link to an script/executable to run. The file must still be
		      explicitly referenced by the command field.`,
			},
		},
		"command": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "Command to run",
				Description: `The first item is the executable to run, followed by command line
					parameters.`,
			},
			Items: schematypes.String{},
		},
	},
}

var configSchema = schematypes.Object{
	Properties: schematypes.Properties{
		"createUser": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title: "Tells if a user should be created to run a command",
				Description: `When set to true, a new user is created on the fly to run
				the command. It runs the command from whitin the user's home directory.`,
			},
		},
		"userGroups": schematypes.Array{
			MetaData: schematypes.MetaData{
				Title: "List of groups to assign user to",
				Description: `It contains the list of groups the user will belong to.
				The first item is the user's primary group, the rest will be user's
				supplementary groups.`,
			},
			Items:  schematypes.String{},
			Unique: true,
		},
		"sudo": schematypes.Boolean{
			MetaData: schematypes.MetaData{
				Title: "Use sudo",
				Description: `Prefix internal privileged commands with sudo.
				This is useful when the worker runs in a non privileged account.`,
			},
		},
	},
	Required: []string{"createUser"},
}

func (e engineProvider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (e *engine) PayloadSchema() schematypes.Object {
	return payloadSchema
}

func (e engine) NewSandboxBuilder(options engines.SandboxOptions) (engines.SandboxBuilder, error) {
	var taskPayload payloadType
	err := payloadSchema.Map(options.Payload, &taskPayload)
	if err == schematypes.ErrTypeMismatch {
		panic("Type mismatch")
	} else if err != nil {
		return nil, engines.NewMalformedPayloadError("Invalid payload: ", err)
	}

	var m sync.Mutex
	return sandboxbuilder{
		SandboxBuilderBase: engines.SandboxBuilderBase{},
		env:                map[string]string{},
		taskPayload:        &taskPayload,
		context:            options.TaskContext,
		envMutex:           &m,
		engine:             &e,
	}, nil
}
