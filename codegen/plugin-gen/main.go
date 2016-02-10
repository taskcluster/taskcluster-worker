// plugin-gen is responsible for auto-generating plugin code, specifically go
// types based on json schema, for both payload and config
package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/jsonschema2go"
)

var (
	version = "plugin-gen 1.0.0"
	usage   = `
plugin-gen
plugin-gen is a tool for generating source code for a plugin, based on two json schema files
(stored as either json or yaml) for its config settings and its portion of the task payload.
As well as creating go types that the payload and config can be unmarshalled into, it also
generates the PayloadSchema() and ConfigSchema() method implementations for the plugin.

  Usage:
    plugin-gen -c CONFIG-SCHEMA-FILE -p PAYLOAD-SCHEMA-FILE -o GO-OUTPUT-FILE
    plugin-gen -h|--help
    plugin-gen --version

  Options:
    -h --help                Display this help text.
    --version                Display the version (` + version + `).
    -c CONFIG-SCHEMA-FILE    The yaml file that describes the structure of the plugin's
                             config, in json schema format.
    -p PAYLOAD-SCHEMA-FILE   The yaml file that describes the structure of the plugin's
                             payload property, in json schema format.
    -o GO-OUTPUT-FILE        The file location to write the generated go code to.
`
)

func main() {
	// Clear all logging fields, such as timestamps etc...
	log.SetFlags(0)
	log.SetPrefix("plugingen: ")

	// Parse the docopt string and exit on any error or help message.
	arguments, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		log.Fatalf("ERROR: Cannot parse arguments: %s\n", err)
	}

	// Avoid multiple `if err != nil` statements by collecting errors into an array
	// and processing at end
	errs := []error{}
	configSchema := getAbsPath(arguments, "-c", errs)
	// payloadSchema := getAbsPath(arguments, "PAYLOAD-SCHEMA-FILE", errs)
	outputFile := getAbsPath(arguments, "-o", errs)

	// log the first error, and exit, if any
	for _, err := range errs {
		log.Fatalf("ERROR: %s", err)
	}

	log.Printf("Generating file %v", outputFile)
	// Get working directory
	currentFolder, err := os.Getwd()
	if err != nil {
		log.Fatalf("Unable to obtain current working directory: %s", err)
	}

	// Read current package
	pkg, err := build.ImportDir(currentFolder, build.AllowBinary)
	if err != nil {
		log.Fatalf("Failed to import current package: %s", err)
	}

	generatedCode, _, err := jsonschema2go.Generate(pkg.Name, "file://"+configSchema)
	if err != nil {
		log.Fatalf("ERROR: Could not interpret file %v as json schema in yaml/json syntax: %s", configSchema, err)
	}

	ioutil.WriteFile(outputFile, generatedCode, 0644)
	if err != nil {
		log.Fatalf("ERROR: Could not write generated source code to file %v: %s", outputFile, err)
	}
}

func getAbsPath(arguments map[string]interface{}, parameter string, errs []error) (absPath string) {
	val, ok := arguments[parameter]
	if !ok {
		errs = append(errs, fmt.Errorf("Parameter %v not specified", parameter))
		return ""
	}
	switch val.(type) {
	case string:
	default:
		errs = append(errs, fmt.Errorf("Parameter %v has type %T but should be string", parameter, parameter))
		return ""
	}
	var err error
	absPath, err = filepath.Abs(val.(string))
	if err != nil {
		errs = append(errs, fmt.Errorf("Parameter %v could not be resolved to an absolute path: %s", parameter, err))
		return ""
	}
	return
}
