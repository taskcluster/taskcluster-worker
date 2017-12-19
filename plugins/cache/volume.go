package cache

import (
	"time"

	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/caching"
)

// cacheVolume is the resource type passed to caching.Cache
type cacheVolume struct {
	Volume   engines.Volume
	Name     string
	Created  time.Time
	disposed atomics.Once
}

func (v *cacheVolume) MemorySize() (uint64, error) {
	// TODO: Add memory size support to cache volumes
	return 0, caching.ErrDisposableSizeNotSupported
}

func (v *cacheVolume) DiskSize() (uint64, error) {
	// TODO: Add disk size support to cache volumes
	return 0, caching.ErrDisposableSizeNotSupported
}

func (v *cacheVolume) Dispose() error {
	var err error
	v.disposed.Do(func() {
		err = v.Volume.Dispose()
	})
	return err
}
