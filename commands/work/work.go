package work

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/taskcluster/taskcluster-worker/commands"
	"github.com/taskcluster/taskcluster-worker/config"
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
  taskcluster-worker work <config.yml>
`
}

func (cmd) Execute(args map[string]interface{}) bool {
	config, err := config.LoadFromFile(args["<config.yml>"].(string))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}

	w, err := worker.New(config, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}

	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	select {
	case <-c:
		signal.Stop(c)
		w.ImmediateStop()
		<-done
	case <-done:
	}

	return true
}
