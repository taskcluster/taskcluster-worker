package volume

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/pborman/uuid"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/plugins"
	"github.com/taskcluster/taskcluster-worker/plugins/extpoints"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type persistentVolumePlugin struct {
	plugins.PluginBase
	mutex       sync.Mutex
	volumes     map[string][]*persistentVolume
	environment *runtime.Environment
	engine      engines.Engine
	log         *logrus.Entry
}

// NewPersistentVolumePlugin is a single plugin that will live between task runs.
// A list of volumes are maintained so that they can be used between task runs
// so long as the volume is not currently claimed by a task.
func NewPersistentVolumePlugin(opts extpoints.PluginOptions) (volumePlugin, error) {
	return &persistentVolumePlugin{
		environment: opts.Environment,
		engine:      opts.Engine,
		log:         opts.Log.WithField("Plugin", "persistent"),
		volumes:     make(map[string][]*persistentVolume),
	}, nil
}

// newVolume will be called by the volume task plugin during the Prepare stage.
// If a previously used volume is available (i.e. not claimed by a task), then
// the volume will be marked as claimed by this task and reused.  If no available
// volume is found, a new volume will be requested from the engine and saved within
// the list of persistent volumes.
func (pv *persistentVolumePlugin) newVolume(opts *volumeOptions) (volume, error) {
	if v := pv.claimAvailableVolume(opts); v != nil {
		return v, nil
	}

	// Create a volume since an available one doesn't exist
	return pv.createVolume(opts)
}

// claimAvailableVolume will claim and return a volume that is found and not currently
// in use.
func (pv *persistentVolumePlugin) claimAvailableVolume(opts *volumeOptions) *persistentVolume {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()
	volumes, _ := pv.volumes[opts.name]
	for _, v := range volumes {
		if !v.isClaimed() {
			v.claim(opts)
			return v
		}
	}
	return nil
}

// createVolume will request a new cache folder from the engine, claim it for the current
// task run, and add it to a list of known persistent volumes.
func (pv *persistentVolumePlugin) createVolume(opts *volumeOptions) (*persistentVolume, error) {
	v := &persistentVolume{
		id:   uuid.New(),
		name: opts.name,
	}

	var err error
	v.volume, err = pv.engine.NewCacheFolder()
	if err != nil {
		e := fmt.Errorf("Could not create persistent volume. %s", err)
		return nil, e
	}

	v.claim(opts)
	pv.addVolume(v)

	return v, nil
}

// addVolume will save a persistent volume so that it can be used for other
// tasks
func (pv *persistentVolumePlugin) addVolume(v *persistentVolume) {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()
	pv.volumes[v.name] = append(pv.volumes[v.name], v)
}

type claim struct {
	taskID     string
	runID      int
	mountPoint string
}

type persistentVolume struct {
	mutex       sync.RWMutex
	id          string
	name        string
	volume      engines.Volume
	claimed     bool
	volumeClaim *claim
}

// isClaimed returns true if the volume is currently in use by a task.
func (v *persistentVolume) isClaimed() bool {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	return v.claimed
}

// claim will reserve the persistent volume for the current task being executed.
func (v *persistentVolume) claim(options *volumeOptions) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.claimed = true
	v.volumeClaim = &claim{
		mountPoint: options.mountPoint,
		taskID:     options.taskID,
		runID:      options.runID,
	}

}

func (v *persistentVolume) String() string {
	return fmt.Sprintf("Persistent Volume '%s' mounted at '%s'", v.name, v.volumeClaim.mountPoint)
}

func (v *persistentVolume) buildSandbox(sb engines.SandboxBuilder) error {
	return sb.AttachVolume(v.volumeClaim.mountPoint, v.volume, false)
}

// dispose will mark a volume as no longer claimed by a task so that it can be used
// for future tasks.
func (v *persistentVolume) dispose() error {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.claimed = false
	v.volumeClaim = &claim{}
	return nil
}
