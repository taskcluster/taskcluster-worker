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
	log *logrus.Entry
}

type engineProvider struct {
	engines.EngineProviderBase
}

func (e engineProvider) NewEngine(options engines.EngineOptions) (engines.Engine, error) {
	return engine{log: options.Log}, nil
}

type payloadType struct {
	Link    string   `json:"link"`
	Command []string `json:"command"`
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
	Required: []string{"command"},
}

func (e engine) PayloadSchema() schematypes.Object {
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
