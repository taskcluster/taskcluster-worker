package volume

import (
	"errors"
	"strings"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type volume interface {
	buildSandbox(engines.SandboxBuilder) error
	dispose() error
	String() string
}

type volumeOptions struct {
	taskID     string
	runID      int
	name       string
	mountPoint string
}

// volumeSet acts as a single task plugin, but is a wrapper around each of the volume
// plugins that were initialized in response to the task payload.
// This plugin will response to typical task plugin events, such as Prepare or Dispose,
// and in turn will call the appropriate methods on each volume plugin for that stage.
type volumeSet struct {
	plugins.TaskPluginBase
	// Group of volumes that should be managed as one during the task life cycle
	volumes []volume
	context *runtime.TaskContext
	payload payload
	vm      *volumeManager
}

func (vs *volumeSet) Prepare(context *runtime.TaskContext) error {
	vs.context = context

	for volumeType, plugin := range vs.vm.plugins {
		switch volumeType {
		case "persistent":
			for _, vol := range vs.payload.Persistent {
				opts := &volumeOptions{
					taskID:     context.TaskInfo.TaskID,
					runID:      context.TaskInfo.RunID,
					name:       vol.Name,
					mountPoint: vol.MountPoint,
				}
				v, err := plugin.newVolume(opts)
				if err != nil {
					return err
				}
				vs.volumes = append(vs.volumes, v)
			}
		default:
			panic(
				"Unrecognized volume type. This should never happen and " +
					"is considered a fatal error.",
			)
		}
	}

	return nil
}

func (vs *volumeSet) BuildSandbox(sandboxBuilder engines.SandboxBuilder) error {
	for _, p := range vs.volumes {
		if err := p.buildSandbox(sandboxBuilder); err != nil {
			return err
		}
	}

	return nil
}

func (vs *volumeSet) Dispose() error {
	errs := []string{}
	for _, p := range vs.volumes {
		if err := p.dispose(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}
