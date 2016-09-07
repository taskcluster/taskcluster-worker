package work

import (
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker"
)

func init() {
	commands.Register("work", cmd{})
}

type cmd struct{}

func (cmd) Summary() string {
	return "Start the worker."
}

func (cmd) Usage() string {
	return `Usage:
  taskcluster-worker work <config-file> [--logging-level <level>]

Options:
  -l --logging-level <level>  	Set logging at <level>.
`
}

func (cmd) Execute(args map[string]interface{}) bool {
	// Setup logger
	var level string
	if l := args["--logging-level"]; l != nil {
		level = l.(string)
	}
	logger, err := runtime.CreateLogger(level)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		return false
	}

	configFile, err := ioutil.ReadFile(args["<config-file>"].(string))
	if err != nil {
		logger.Error("Failed to open configFile, error: ", err)
		return false
	}
	var config interface{}
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		logger.Error("Failed to parse configFile, error: ", err)
		return false
	}

	// Create a temporary folder
	tempPath := filepath.Join(os.TempDir(), slugid.Nice())
	tempStorage, err := runtime.NewTemporaryStorage(tempPath)
	runtimeEnvironment := &runtime.Environment{
		Log:              logger,
		TemporaryStorage: tempStorage,
	}

	w, err := worker.New(config, runtimeEnvironment)
	if err != nil {
		logger.Fatalf("Could not create worker. %s", err)
	}

	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	select {
	case <-c:
		signal.Stop(c)
		w.Stop()
		<-done
	case <-done:
	}

	return true
}
