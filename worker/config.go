package worker

import (
	"math"

	schematypes "github.com/taskcluster/go-schematypes"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
	"github.com/taskcluster/taskcluster-worker/runtime/webhookserver"
)

type options struct {
	ProvisionerID       string `json:"provisionerId"`
	WorkerType          string `json:"workerType"`
	WorkerGroup         string `json:"workerGroup"`
	WorkerID            string `json:"workerId"`
	PollingInterval     int    `json:"pollingInterval"`
	ReclaimOffset       int    `json:"reclaimOffset"`
	MinimumReclaimDelay int    `json:"minimumReclaimDelay"`
	Concurrency         int    `json:"concurrency"`
}

type configType struct {
	Engine           string                 `json:"engine"`
	EngineConfig     map[string]interface{} `json:"engines"`
	Plugins          interface{}            `json:"plugins"`
	WebHookServer    interface{}            `json:"webHookServer"`
	TemporaryFolder  string                 `json:"temporaryFolder"`
	MinimumDiskSpace int64                  `json:"minimumDiskSpace"`
	MinimumMemory    int64                  `json:"minimumMemory"`
	Monitor          interface{}            `json:"monitor"`
	Credentials      tcclient.Credentials   `json:"credentials"`
	QueueBaseURL     string                 `json:"queueBaseUrl"`
	AuthBaseURL      string                 `json:"authBaseUrl"`
	WorkerOptions    options                `json:"worker"`
}

// optionsSchema must be satisfied by Options used to construct a Worker
var optionsSchema schematypes.Schema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title:       "Worker Config",
		Description: "Configuration for the worker",
	},
	Properties: schematypes.Properties{
		"provisionerId": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "ProvisionerId",
				Description: util.Markdown(`
					ProvisionerId for workerType that tasks should be claimed
					from. Note, a 'workerType' is only unique given the 'provisionerId'.
				`),
			},
			Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
		},
		"workerType": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "WorkerType",
				Description: util.Markdown(`
					WorkerType to claim tasks for, combined with 'provisionerId' this
					identifies the pool of workers the machine belongs to.
				`),
			},
			Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
		},
		"workerGroup": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "WorkerGroup",
				Description: util.Markdown(`
					Group of workers this machine belongs to. This is any identifier such
					that workerGroup and workerId uniquely identifies this machine.
				`),
			},
			Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
		},
		"workerId": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "WorkerId",
				Description: util.Markdown(`
					Identifier for this machine. This is any identifier such
					that workerGroup and workerId uniquely identifies this machine.
				`),
			},
			Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
		},
		"pollingInterval": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Task Polling Interval",
				Description: util.Markdown(`
					The amount of time to wait between task polling
					iterations in seconds.
				`),
			},
			Minimum: 0,
			Maximum: 10 * 60,
		},
		"reclaimOffset": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Reclaim Offset",
				Description: util.Markdown(`
					The number of seconds prior to task claim expiration the
					claim should be reclamed.
				`),
			},
			Minimum: 0,
			Maximum: 10 * 60,
		},
		"minimumReclaimDelay": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title: "Minimum Reclaim Delay",
				Description: util.Markdown(`
					Minimum number of seconds to wait before reclaiming a task.
					it is important that this is some reasonable non-zero minimum to avoid
					overloading servers if there is some error.
				`),
			},
			Minimum: 0,
			Maximum: 10 * 60,
		},
		"concurrency": schematypes.Integer{
			MetaData: schematypes.MetaData{
				Title:       "Concurrency",
				Description: "The number of tasks that this worker supports running in parallel.",
			},
			Minimum: 1,
			Maximum: 1000,
		},
	},
	Required: []string{
		"provisionerId",
		"workerType",
		"workerGroup",
		"workerId",
		"pollingInterval",
		"reclaimOffset",
		"minimumReclaimDelay",
		"concurrency",
	},
}

var credentialsSchema schematypes.Schema = schematypes.Object{
	MetaData: schematypes.MetaData{
		Title: "TaskCluster Credentials",
		Description: util.Markdown(`
			The set of credentials that should be used by the worker
			when authenticating against taskcluster endpoints. This needs scopes
			for claiming tasks for the given workerType.
		`),
	},
	Properties: schematypes.Properties{
		"clientId": schematypes.String{
			MetaData: schematypes.MetaData{
				Title:       "ClientId",
				Description: `ClientId for credentials`,
			},
			Pattern: `^[A-Za-z0-9@/:._-]+$`,
		},
		"accessToken": schematypes.String{
			MetaData: schematypes.MetaData{
				Title:       "AccessToken",
				Description: `The security-sensitive access token for the client.`,
			},
			Pattern: `^[a-zA-Z0-9_-]{22,66}$`,
		},
		"certificate": schematypes.String{
			MetaData: schematypes.MetaData{
				Title: "Certificate",
				Description: util.Markdown(`
					The certificate for the client, if using temporary credentials.
				`),
			},
		},
		"authorizedScopes": schematypes.Array{
			Items: schematypes.String{},
		},
	},
	Required: []string{"clientId", "accessToken"},
}

// ConfigSchema returns the schema for configuration.
func ConfigSchema() schematypes.Object {
	engineConfig := schematypes.Properties{}
	engineNames := []string{}
	for name, provider := range engines.Engines() {
		engineNames = append(engineNames, name)
		engineConfig[name] = provider.ConfigSchema()
	}
	return schematypes.Object{
		Properties: schematypes.Properties{
			"engine": schematypes.StringEnum{
				MetaData: schematypes.MetaData{
					Title: "Worker Engine",
					Description: util.Markdown(`
						Selected worker engine to use, notice that the
						configuration for this engine **must** be present under the
						'engines.<engine>' configuration key.
					`),
				},
				Options: engineNames,
			},
			"engines": schematypes.Object{
				MetaData: schematypes.MetaData{
					Title: "Engine Configuration",
					Description: util.Markdown(`
						Mapping from engine name to engine configuration.
						Even-though the worker will only use one engine at any given time,
						the configuration file can hold configuration for all engines.
						Hence, you need only update the 'engine' key to change which engine
						should be used.
					`),
				},
				Properties: engineConfig,
			},
			"plugins":       plugins.PluginManagerConfigSchema(),
			"webHookServer": webhookserver.ConfigSchema,
			"temporaryFolder": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "Temporary Folder",
					Description: util.Markdown(`
						Path to folder that can be used for temporary files and
						folders, if folder doesn't exist it will be created, otherwise it
						will be overwritten.
					`),
				},
			},
			"minimumDiskSpace": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Minimum Disk Space",
					Description: util.Markdown(`
						The minimum amount of disk space in bytes to have available
						before starting on the next task. Garbage collector will do a
						best-effort attempt at releasing resources to satisfy this limit.
					`),
				},
				Minimum: 0,
				Maximum: math.MaxInt64,
			},
			"minimumMemory": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Minimum Memory",
					Description: util.Markdown(`
						The minimum amount of memory in bytes to have available
						before starting on the next task. Garbage collector will do a
						best-effort attempt at releasing resources to satisfy this limit.
					`),
				},
				Minimum: 0,
				Maximum: math.MaxInt64,
			},
			"monitor":      monitoring.ConfigSchema,
			"credentials":  credentialsSchema,
			"queueBaseUrl": schematypes.String{},
			"authBaseUrl":  schematypes.String{},
			"worker":       optionsSchema,
		},
		Required: []string{
			"engine",
			"engines",
			"plugins",
			"webHookServer",
			"temporaryFolder",
			"minimumDiskSpace",
			"minimumMemory",
			"monitor",
			"credentials",
			"worker",
		},
	}
}
