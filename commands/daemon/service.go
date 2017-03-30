package daemon

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/takama/daemon"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/worker"
)

// service has embedded daemon
type service struct {
	daemon.Daemon
	args map[string]interface{}
}

func (svc *service) Run() (string, error) {
	logger := logrus.New()
	err := setupSyslog(logger)
	if err != nil {
		return "Could not create syslog", err
	}

	// load configuration file
	config, err := config.LoadFromFile(svc.args["<config-file>"].(string))
	if err != nil {
		logger.WithError(err).Error("Failed to open configuration file")
		return "Failed to open configuration file", err
	}

	w, err := worker.New(config)
	if err != nil {
		logger.WithError(err).Error("Could not create worker")
		return "Could not create worker", err
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigTerm
		w.StopNow()
	}()

	w.Start()
	return "Worker successfully started", nil
}

// Manage by daemon commands or run the daemon
func (svc *service) Manage() (string, error) {
	// if received any kind of command, do it
	if svc.args["install"].(bool) {
		args := []string{"daemon", "run", svc.args["<config-file>"].(string)}
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
