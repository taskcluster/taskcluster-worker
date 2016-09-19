package worker

import (
	"fmt"
	"io/ioutil"
	"math"

	"github.com/Sirupsen/logrus"
	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	yaml "gopkg.in/yaml.v2"
)

type configType struct {
	Engine           string                 `json:"engine"`
	Engines          map[string]interface{} `json:"engines"`
	Plugins          interface{}            `json:"plugins"`
	Capacity         int                    `json:"capacity"`
	Credentials      credentials            `json:"credentials"`
	PollingInterval  int                    `json:"pollingInterval"`
	ReclaimOffset    int                    `json:"reclaimOffset"`
	QueueBaseURL     string                 `json:"queueBaseUrl"`
	ProvisionerID    string                 `json:"provisionerId"`
	WorkerType       string                 `json:"workerType"`
	WorkerGroup      string                 `json:"workerGroup"`
	WorkerID         string                 `json:"workerId"`
	TemporaryFolder  string                 `json:"temporaryFolder"`
	LogLevel         string                 `json:"logLevel"`
	ServerIP         string                 `json:"serverIp"`
	ServerPort       int                    `json:"serverPort"`
	TLSCertificate   string                 `json:"tlsCertificiate"`
	TLSKey           string                 `json:"tlsKey"`
	DNSSecret        string                 `json:"statelessDNSSecret"`
	DNSDomain        string                 `json:"statelessDNSDomain"`
	MaxLifeCycle     int                    `json:"maxLifeCycle"`
	MinimumDiskSpace int64                  `json:"minimumDiskSpace"`
	MinimumMemory    int64                  `json:"minimumMemory"`
}

type credentials struct {
	ClientID    string `json:"clientId"`
	AccessToken string `json:"accessToken"`
	Certificate string `json:"certificate"`
}

// ConfigSchema returns the configuration schema for the worker.
func ConfigSchema() schematypes.Object {
	engineConfig := schematypes.Properties{}
	engineNames := []string{}
	for name, provider := range engines.Engines() {
		engineNames = append(engineNames, name)
		engineConfig[name] = provider.ConfigSchema()
	}
	return schematypes.Object{
		MetaData: schematypes.MetaData{
			Title:       "Worker Configuration",
			Description: `This contains configuration for the worker process.`,
		},
		Properties: schematypes.Properties{
			"engine": schematypes.StringEnum{
				MetaData: schematypes.MetaData{
					Title: "Worker Engine",
					Description: `Selected worker engine to use, notice that the
						configuration for this engine **must** be present under the
						'engines.<engine>' configuration key.`,
				},
				Options: engineNames,
			},
			"engines": schematypes.Object{
				MetaData: schematypes.MetaData{
					Title: "Engine Configuration",
					Description: `Mapping from engine name to engine configuration.
						Even-though the worker will only use one engine at any given time,
						the configuration file can hold configuration for all engines.
						Hence, you need only update the 'engine' key to change which engine
						should be used.`,
				},
				Properties: engineConfig,
			},
			"plugins": plugins.PluginManagerConfigSchema(),
			"capacity": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Capacity",
					Description: `The number of tasks that this worker supports running in
          parallel.`,
				},
				Minimum: 1,
				Maximum: 1000,
			},
			"credentials": schematypes.Object{
				MetaData: schematypes.MetaData{
					Title: "TaskCluster Credentials",
					Description: `The set of credentials that should be used by the worker
          when authenticating against taskcluster endpoints. This needs scopes
          for claiming tasks for the given workerType.`,
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
							Description: `The certificate for the client, if using temporary
              credentials.`,
						},
					},
				},
				Required: []string{"clientId", "accessToken"},
			},
			"pollingInterval": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Task Polling Interval",
					Description: `The amount of time to wait between task polling
          iterations in seconds.`,
				},
				Minimum: 0,
				Maximum: 10 * 60,
			},
			"reclaimOffset": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Reclaim Offset",
					Description: `The number of seconds priorty task claim expiration the
          claim should be reclamed.`,
				},
				Minimum: 0,
				Maximum: 10 * 60,
			},
			"queueBaseUrl": schematypes.URI{
				MetaData: schematypes.MetaData{
					Title: "Queue BaseUrl",
					Description: `BaseUrl for taskcluster-queue, defaults to value from the
          taskcluster client library.`,
				},
			},
			"provisionerId": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "ProvisionerId",
					Description: `ProvisionerId for workerType that tasks should be claimed
          from. Note, a 'workerType' is only unique given the 'provisionerId'.`,
				},
				Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
			},
			"workerType": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "WorkerType",
					Description: `WorkerType to claim tasks for, combined with
          'provisionerId' this identifies the pool of workers the machine
          belongs to.`,
				},
				Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
			},
			"workerGroup": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "WorkerGroup",
					Description: `Group of workers this machine belongs to. This is any
          identifier such that workerGroup and workerId uniquely identifies this
          machine.`,
				},
				Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
			},
			"workerId": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "WorkerId",
					Description: `Identifier for this machine. This is any identifier such
          that workerGroup and workerId uniquely identifies this machine.`,
				},
				Pattern: `^[a-zA-Z0-9_-]{1,22}$`,
			},
			"temporaryFolder": schematypes.String{
				MetaData: schematypes.MetaData{
					Title: "Temporary Folder",
					Description: `Path to folder that can be used for temporary files and
							folders, if folder doesn't exist it will be created, it will be
							overwritten.`,
				},
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
			"serverIp": schematypes.String{},
			"serverPort": schematypes.Integer{
				Minimum: 0,
				Maximum: 65535,
			},
			"tlsCertificiate":    schematypes.String{},
			"tlsKey":             schematypes.String{},
			"statelessDNSSecret": schematypes.String{},
			"statelessDNSDomain": schematypes.String{},
			"maxLifeCycle": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title:       "Max life cycle of worker",
					Description: "Used to limit validity of hostname",
				},
				Minimum: 5 * 60,
				Maximum: 31 * 24 * 60 * 60,
			},
			"minimumDiskSpace": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Minimum Disk Space",
					Description: `The minimum amount of disk space to have available
						before starting on the next task. Garbage collector will do a
						best-effort attempt at releasing resources to satisfy this limit`,
				},
				Minimum: 0,
				Maximum: math.MaxInt64,
			},
			"minimumMemory": schematypes.Integer{
				MetaData: schematypes.MetaData{
					Title: "Minimum Memory",
					Description: `The minimum amount of memory to have available
						before starting on the next task. Garbage collector will do a
						best-effort attempt at releasing resources to satisfy this limit`,
				},
				Minimum: 0,
				Maximum: math.MaxInt64,
			},
		},
		Required: []string{
			"engine",
			"engines",
			"plugins",
			"capacity",
			"credentials",
			"pollingInterval",
			"reclaimOffset",
			"provisionerId",
			"workerType",
			"workerGroup",
			"workerId",
			"temporaryFolder",
			"logLevel",
			"serverIp",
			"serverPort",
			"statelessDNSSecret",
			"statelessDNSDomain",
			"maxLifeCycle",
			"minimumDiskSpace",
			"minimumMemory",
		},
	}
}

// LoadConfigFile will load configuration options from a YAML file and validate
// against the config file schema, returning an error messages explaining what
// went wrong if unsuccessful.
func LoadConfigFile(filename string) (interface{}, error) {
	// Read config file
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to read config file: '%s' error: %s\n",
			filename, err)
	}
	// Parse config file
	var config interface{}
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse YAML from config file: '%s', error: %s\n",
			filename, err)
	}
	// This fixes obscurities in yaml.Unmarshal where it generates
	// map[interface{}]interface{} instead of map[string]interface{}
	// credits: https://github.com/go-yaml/yaml/issues/139#issuecomment-220072190
	config = convertToMapStr(config)
	// Validate configuration file against schema
	err = ConfigSchema().Validate(config)
	if err != nil {
		return nil, fmt.Errorf("Invalid configuration options, error: %s\n", err)
	}
	return config, nil
}

func convertToMapStr(val interface{}) interface{} {
	switch val := val.(type) {
	case []interface{}:
		r := make([]interface{}, len(val))
		for i, v := range val {
			r[i] = convertToMapStr(v)
		}
		return r
	case map[interface{}]interface{}:
		r := make(map[string]interface{})
		for k, v := range val {
			s, ok := k.(string)
			if !ok {
				s = fmt.Sprintf("%v", k)
			}
			r[s] = convertToMapStr(v)
		}
		return r
	default:
		return val
	}
}
