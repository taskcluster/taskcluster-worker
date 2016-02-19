// go-composite-schema is responsible for auto-generating plugin and engine code based
// on static yml json schema files
package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/ghodss/yaml"
	"github.com/kr/text"
	"github.com/taskcluster/jsonschema2go"
	"github.com/taskcluster/taskcluster-base-go/jsontest"
	"golang.org/x/tools/imports"
)

var (
	version = "go-composite-schema 1.0.0"
	usage   = `
go-composite-schema
go-composite-schema is a tool for generating go source code for a function to
return a CompositeSchema based on a static json schema definition stored in a
yaml/json file in a package directory of a go project, together with some
parameters included in a "go:generate" command. See
https://godoc.org/github.com/taskcluster/taskcluster-worker/runtime#CompositeSchema
for more information.

Although the go-composite-schema command line tool can be used wherever you
require a CompositeSchema, it is currently most appliable to taskcluster-worker
engines and plugins.

Typically, you should include a code generation comment in your source code for
each CompositeSchema returning function you wish to create:

 //go:generate go-composite-schema [--required] PROPERTY INPUT-FILE OUTPUT-FILE

We currently use CompositeSchemas for defining both config structures and
payload structures, for both engines and plugins. Therefore typically there
could be one or two "go:generate" lines required per plugin and probably two
per engine. This is because some plugins might not have custom config nor even
custom payload, but engines are likely to require both.

Please note, it is recommended to set environment variable GOPATH in order for
go-composite-schema to correctly determine the correct package name.


  Usage:
    go-composite-schema [--required] PROPERTY INPUT-FILE OUTPUT-FILE
    go-composite-schema -h|--help
    go-composite-schema --version

  Options:
    -h --help              Display this help text.
    --version              Display the version (` + version + `).
`
)

func main() {
	// Clear all logging fields, such as timestamps etc...
	log.SetFlags(0)
	log.SetPrefix("go-composite-schema: ")

	// Parse the docopt string and exit on any error or help message.
	args, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		log.Fatalf("ERROR: Cannot parse arguments: %s\n", err)
	}

	// assuming non-nil, and always type bool
	req := args["--required"].(bool)

	// assuming non-nil, and always type string
	schemaProperty := args["PROPERTY"].(string)
	inputFile := args["INPUT-FILE"].(string)
	outputFile := args["OUTPUT-FILE"].(string)

	// Get working directory
	currentFolder, err := os.Getwd()
	if err != nil {
		log.Fatalf("Unable to obtain current working directory: %s", err)
	}

	// Read current package
	pkg, err := build.ImportDir(currentFolder, build.AllowBinary)
	if err != nil {
		log.Fatalf("ERROR: Failed to determine go package inside directory '%s' - is your GOPATH set correctly ('%s')? Error: %s", currentFolder, os.Getenv("GOPATH"), err)
	}

	// Generate go types...
	ymlFile := filepath.Join(currentFolder, inputFile)
	if _, err := os.Stat(ymlFile); err == nil {
		log.Printf("Found yaml file '%v'", ymlFile)
	} else {
		log.Fatalf("ERROR: could not read file '%v'", ymlFile)
	}
	url := "file://" + ymlFile
	goFile := filepath.Join(currentFolder, outputFile)
	log.Printf("Generating '%v'...", goFile)
	job := &jsonschema2go.Job{
		Package:     pkg.Name,
		URLs:        []string{url},
		ExportTypes: false,
	}
	result, err := job.Execute()
	if err != nil {
		log.Fatalf("ERROR: Problem assembling content for file '%v': %s", goFile, err)
	}
	generatedCode := append(result.SourceCode, []byte("\n"+generateFunctions(ymlFile, result.SchemaSet.SubSchema(url).TypeName, schemaProperty, req))...)
	sourceCode, err := imports.Process(
		goFile,
		[]byte(generatedCode),
		&imports.Options{
			AllErrors: true,
			Comments:  true,
			TabIndent: true,
			TabWidth:  0,
			Fragment:  false,
		},
	)
	if err != nil {
		log.Fatalf("ERROR: Could not format generated source code for file '%v': %s\nCode:\n%v", goFile, err, string(generatedCode))
	}
	ioutil.WriteFile(goFile, []byte(sourceCode), 0644)
	if err != nil {
		log.Fatalf("ERROR: Could not write generated source code to file '%v': %s", goFile, err)
	}
}

func generateFunctions(ymlFile, goType, schemaProperty string, req bool) string {
	data, err := ioutil.ReadFile(ymlFile)
	if err != nil {
		log.Fatalf("ERROR: Problem reading from file '%v' - %s", ymlFile, err)
	}
	// json is valid YAML, so we can safely convert, even if it is already json
	rawJson, err := yaml.YAMLToJSON(data)
	if err != nil {
		log.Fatalf("ERROR: Problem converting file '%v' to json format - %s", ymlFile, err)
	}
	rawJson, err = jsontest.FormatJson(rawJson)
	if err != nil {
		log.Fatalf("ERROR: Problem pretty printing json in '%v' - %s", ymlFile, err)
	}
	result := "func " + goType + "Schema() runtime.CompositeSchema {\n"
	result += "\tschema, err := runtime.NewCompositeSchema(\n"
	result += "\t\t\"" + schemaProperty + "\",\n"
	result += "\t\t`\n"
	// the following strings.Replace function call safely escapes backticks (`) in rawJson
	result += strings.Replace(text.Indent(fmt.Sprintf("%v", string(rawJson)), "\t\t")+"\n", "`", "` + \"`\" + `", -1)
	result += "\t\t`,\n"
	if req {
		result += "\t\ttrue,\n"
	}
	result += "\t\tfunc() interface{} {\n"
	result += "\t\t\treturn &" + goType + "{}\n"
	result += "\t\t},\n"
	result += "\t)\n"
	result += "\tif err != nil {\n"
	result += "\t\tpanic(err)\n"
	result += "\t}\n"
	result += "\treturn schema\n"
	result += "}\n"
	return result
}
