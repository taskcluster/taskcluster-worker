package schema

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/worker"
)

func init() {
	commands.Register("schema", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Dump schema for config or payload"
}

func (cmd) Usage() string {
	return `
taskcluster-worker schema can be used to export JSON schema document
for the worker configuration file. Given a configuration file the command can
also be used to export payload schema.

usage:
  taskcluster-worker schema config [options]
  taskcluster-worker schema payload [options] <config.yml>

options:
  -f --format <format>          Set the format json or yaml [Default: json].
  -o --output <file>            Write output to a file [Default: -].
`
}

func (cmd) Execute(args map[string]interface{}) bool {
	var schema interface{}

	if args["config"].(bool) {
		schema = worker.ConfigSchema().Schema()
	} else {
		config, err := worker.LoadConfigFile(args["<config.yml>"].(string))
		if err != nil {
			fmt.Println(err)
			return false
		}
		// Create worker instance
		w, err := worker.New(config, nil)
		if err != nil {
			fmt.Printf("Failed to initialize worker, error: %s\n", err)
			return false
		}
		schema = w.PayloadSchema().Schema()
	}

	// Format schema to JSON or YAML
	var data []byte
	var err error
	if args["--format"].(string) == "yaml" {
		data, err = yaml.Marshal(schema)
	} else {
		data, err = json.MarshalIndent(schema, "", "  ")
	}
	if err != nil {
		panic(fmt.Sprintf("Internal error, failed to serialize, error: %s", err))
	}

	// Write output file or write to stdout
	output := args["--output"].(string)
	if output != "-" {
		err = ioutil.WriteFile(output, data, 0777)
		if err != nil {
			fmt.Printf("Failed to write file: '%s', error: %s\n", output, err)
			return false
		}
	} else {
		fmt.Println(string(data))
	}

	return true
}
