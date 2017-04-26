package config

import (
	"fmt"
	"io/ioutil"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker"

	yaml "gopkg.in/yaml.v2"
)

// Schema returns the configuration file schema
func Schema() schematypes.Object {
	transformations := []string{}
	for name := range Providers() {
		transformations = append(transformations, name)
	}
	s := schematypes.Object{
		Title:       "Worker Configuration",
		Description: `Initial configuration and transformations to run.`,
		Properties: schematypes.Properties{
			"transforms": schematypes.Array{
				Title:       "Configuration Transformations",
				Description: "Ordered list of transformations to run on the config.",
				Items: schematypes.StringEnum{
					Options: transformations,
				},
			},
			"config": worker.ConfigSchema(),
		},
		Required: []string{"config"},
	}
	return s
}

// Load configuration from YAML config object.
func Load(data []byte, monitor runtime.Monitor) (map[string]interface{}, error) {
	// Parse config file
	var config interface{}
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse YAML config, error: %s", err)
	}
	// This fixes obscurities in yaml.Unmarshal where it generates
	// map[interface{}]interface{} instead of map[string]interface{}
	// credits: https://github.com/go-yaml/yaml/issues/139#issuecomment-220072190
	config = convertSimpleJSONTypes(config)

	// Extract transforms and config
	c, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Expected top-level config value to be an object")
	}
	result, ok := c["config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Expected 'config' property to be an object")
	}

	// Ensure that we have a simple JSON compatible structure
	if err := jsonCompatTypes(result); err != nil {
		panic(fmt.Sprintf("YAML loaded wrong types, error: %s", err))
	}

	// Apply transforms
	if ct, ok := c["transforms"]; ok {
		var transforms []string
		err := schematypes.MustMap(Schema().Properties["transforms"], ct, &transforms)
		if err != nil {
			return nil, fmt.Errorf("'transforms' schema violated, error: %s", err)
		}

		providers := Providers()
		for _, t := range transforms {
			provider, ok := providers[t]
			if !ok {
				return nil, fmt.Errorf("Unknown config transformation: %s", t)
			}
			if err := provider.Transform(result, monitor); err != nil {
				return nil, fmt.Errorf("Config transformation: %s failed error: %s",
					t, err)
			}

			// Ensure that transform only injects simple JSON compatible types
			if err := jsonCompatTypes(result); err != nil {
				panic(fmt.Sprintf("%s injected wrong types, error: %s", t, err))
			}
		}
	}

	// Filter out keys that aren't in the config schema...
	// This way extra keys can be used to provide options for the
	// transformations, like "secrets" which will use the secretsBaseUrl if
	// present in the configuration.
	worker.ConfigSchema().Filter(result)

	// Validate against worker schema
	if err := worker.ConfigSchema().Validate(result); err != nil {
		return nil, err
	}

	return result, nil
}

// LoadFromFile will load configuration options from a YAML file and validate
// against the config file schema, returning an error message explaining what
// went wrong if unsuccessful.
func LoadFromFile(filename string, monitor runtime.Monitor) (interface{}, error) {
	// Read config file
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %s", filename, err)
	}

	return Load(configFile, monitor)
}

func convertSimpleJSONTypes(val interface{}) interface{} {
	switch val := val.(type) {
	case []interface{}:
		r := make([]interface{}, len(val))
		for i, v := range val {
			r[i] = convertSimpleJSONTypes(v)
		}
		return r
	case map[interface{}]interface{}:
		r := make(map[string]interface{})
		for k, v := range val {
			s, ok := k.(string)
			if !ok {
				s = fmt.Sprintf("%v", k)
			}
			r[s] = convertSimpleJSONTypes(v)
		}
		return r
	case int:
		return float64(val)
	default:
		return val
	}
}
