// plugin-gen is responsible for auto-generating plugin code, specifically go
// types based on json schema, for both payload and config
package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/taskcluster/jsonschema2go"
)

var (
	version = "plugin-gen 1.0.0"
	usage   = `
plugin-gen
plugin-gen is a tool for generating source code for a plugin. It is designed to
be run from inside a plugin package of a taskcluster-worker source code
directory (package). It then generates go source code files in the current
directory, based on the files it discovers.

If config-schema.yml exists, it is assumed to store the json schema (in
yml/json format) of the config used by this plugin. The following files will
then be generated in the current directory:

  * configtypes.go:  Generated go type(s) for the plugin's config.
  * configschema.go: Generated ConfigSchema() method that returns the
    plugin's config schema in json format, as a []byte.

If payload-schema.yml exists, and PAYLOAD-PROPERTY has been specified, the file
is assumed to store the json schema (in yml/json format) of the property
PAYLOAD-PROPERTY that appears in the payload section of the task definition.
The following files will then be generated in the current directory:

  * payloadtypes.go:  Generated go type(s) for the plugin's payload property.
  * payloadschema.go: Generated PayloadSchema() method that returns the
    plugin's payload schema in json format, as a []byte, together with payload
    json property name that the schema relates to.

Note, since plugins may not require config nor task payload data, it is not
necessary for either config-schema.yml nor payload-schema.yml to exist.

Please also note, it is recommended to set environment variable GOPATH in order
for plugin-gen to correctly determine the correct package name.


  Usage:
    plugin-gen [-p PAYLOAD-PROPERTY]
    plugin-gen -h|--help
    plugin-gen --version

  Options:
    -h --help              Display this help text.
    --version              Display the version (` + version + `).
    -p PAYLOAD-PROPERTY    The payload property that relates to the plugin.

  Examples:
    plugin-gen -p livelog
    plugin-gen --version
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

	payloadProperty := ""
	prop, ok := arguments["-p"]
	if ok {
		// ensure parameter is rendered as a string
		// e.g. in case it gets resolved as bool true/false
		payloadProperty := fmt.Sprintf("%s", prop)
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
	return
}
