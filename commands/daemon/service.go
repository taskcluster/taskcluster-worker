package daemon

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/takama/daemon"
	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/monitoring"
	"github.com/taskcluster/taskcluster-worker/worker"
)

// service has embedded daemon
type service struct {
	daemon.Daemon
	args map[string]interface{}
}

func (svc *service) Run(monitor runtime.Monitor) (string, error) {
	// load configuration file
	config, err := config.LoadFromFile(svc.args["<config-file>"].(string), monitor)
	if err != nil {
		monitor.ReportError(err, "Failed to open configuration file")
		return "Failed to open configuration file", err
	}

	w, err := worker.New(config)
	if err != nil {
		monitor.ReportError(err, "Could not create worker")
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
	monitor := monitoring.PreConfig()

	// if received any kind of command, do it
	if svc.args["install"].(bool) {
		args := []string{"daemon", "run", svc.args["<config-file>"].(string)}
		monitor.Info("installing daemon")
		return svc.Install(args...)
	}

	if svc.args["remove"].(bool) {
		monitor.Info("removing daemon")
		return svc.Remove()
	}

	if svc.args["start"].(bool) {
		monitor.Info("starting daemon")
		return svc.Start()
	}

	if svc.args["stop"].(bool) {
		monitor.Info("stopping daemon")
		return svc.Stop()
	}

	if svc.args["run"].(bool) {
		monitor.Info("running daemon")
		return svc.Run(monitor)
	}

	return usage(), nil
}
