package config

import (
	"fmt"
	"io/ioutil"

	schematypes "github.com/taskcluster/go-schematypes"

	yaml "gopkg.in/yaml.v2"
)

// Schema returns the configuration file schema
func Schema() schematypes.Schema {
	s := schematypes.Array{}
	return s
}

// LoadFromFile will load configuration options from a YAML file and validate
// against the config file schema, returning an error message explaining what
// went wrong if unsuccessful.
func LoadFromFile(filename string) (interface{}, error) {
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
	err = Schema().Validate(config)
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
