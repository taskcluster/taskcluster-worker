package daemon

import (
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	yaml "gopkg.in/yaml.v2"

	"github.com/Sirupsen/logrus"
	"github.com/takama/daemon"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker"
)

// service has embedded daemon
type service struct {
	daemon.Daemon
	args map[string]interface{}
	log  *logrus.Logger
}

func (svc *service) Run() (string, error) {
	logger := svc.log
	err := setupSyslog(logger)
	if err != nil {
		return "Could not create syslog", err
	}

	// load configuration file
	configFile, err := ioutil.ReadFile(svc.args["<config-file>"].(string))
	if err != nil {
		return "Failed to open configFile", err
	}
	var config interface{}
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return "Failed to parse configFile", err
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
		return "Could not create worker", err
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	go func() {
		<-sigTerm
		w.Stop()
	}()
	w.Start()
	return "Worker successfully started", nil
}

// Manage by daemon commands or run the daemon
func (svc *service) Manage() (string, error) {
	// if received any kind of command, do it
	if svc.args["install"].(bool) {
		args := []string{"daemon", "run", svc.args["<engine>"].(string)}
		for _, a := range []string{"--logging-level"} {
			args = append(args, a, svc.args[a].(string))
		}
		return svc.Install(args...)
	}

	if svc.args["remove"].(bool) {
		return svc.Remove()
	}

	if svc.args["start"].(bool) {
		return svc.Start()
	}

	if svc.args["stop"].(bool) {
		return svc.Stop()
	}

	if svc.args["run"].(bool) {
		return svc.Run()
	}

	return usage(), nil
}
