package gc

import (
	"sort"
	"sync"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

func indexOfResource(resources []Disposable, resource Disposable) int {
	for i, r := range resources {
		if r == resource {
			return i
		}
	}
	return -1
}

type disposableSorter []Disposable

func (d disposableSorter) Len() int {
	return len(d)
}

func (d disposableSorter) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d disposableSorter) Less(i, j int) bool {
	return d[i].LastUsed().Before(d[j].LastUsed())
}

// A ResourceTracker is an object capable of tracking resources.
//
// This is the interface for the GarbageCollector that should be exposed to
// engines and plugins. So they can't initiate garbage collection.
type ResourceTracker interface {
	Register(resource Disposable)
	Unregister(resource Disposable) bool
}

// GarbageCollector can be used register Disposable resources which will then
// be diposed when not in use and the system is low on available disk space
// or memory.
type GarbageCollector struct {
	resources        []Disposable
	m                sync.Mutex
	storageFolder    string
	minimumDiskSpace int64
	minimumMemory    int64
}

// New creates a GarbageCollector which uses storageFolder to test for available
// diskspace and tries to ensure that minimumDiskSpace and minimumMemory is
// satisfied after each call to Collect()
func New(storageFolder string, minimumDiskSpace, minimumMemory int64) *GarbageCollector {
	return &GarbageCollector{
		storageFolder:    storageFolder,
		minimumDiskSpace: minimumDiskSpace,
		minimumMemory:    minimumMemory,
	}
}

// Register takes a Disposable resource for the GarbageCollector to manage.
//
// GarbageCollector will attempt to to call resource.Dispose() at any time,
// you should return ErrDisposableInUse if the resource is in use.
func (gc *GarbageCollector) Register(resource Disposable) {
	gc.m.Lock()
	defer gc.m.Unlock()
	if indexOfResource(gc.resources, resource) == -1 {
		gc.resources = append(gc.resources, resource)
	}
}

// Unregister will inform the GarbageCollector to stop tracking the given
// resource. Returns true if the resource was tracked, notice that the resource
// maybe have been disposed of while waiting for the GC lock. Say if the GC was
// running when you made this call.
//
// Note, you don't have to use this method. When you resource is in a state
// where you don't want it to be disposed just ensure that Dispose() returns
// ErrDisposableInUse.
func (gc *GarbageCollector) Unregister(resource Disposable) bool {
	gc.m.Lock()
	defer gc.m.Unlock()
	i := indexOfResource(gc.resources, resource)
	if i != -1 {
		gc.resources = append(gc.resources[:i], gc.resources[i+1:]...)
		return true
	}
	return false
}

// Collect runs garbage collection and reclaims resources, attempting to
// satisfy minimumMemory and minimumDiskSpace, if possible.
func (gc *GarbageCollector) Collect() error {
	gc.m.Lock()
	defer gc.m.Unlock()

	// Sort to get least-recently-used first
	sort.Sort(disposableSorter(gc.resources))

	var resources []Disposable
	for i, r := range gc.resources {
		var err error
		var size uint64

		abort := false
		dispose := false
		if gc.needDiskSpace() {
			size, err = r.DiskSize()
			if err != nil && err != ErrDisposableSizeNotSupported {
				abort = true
			} else if size > 0 || err == ErrDisposableSizeNotSupported {
				dispose = true
			}
		}

		if !abort && !dispose && gc.needMemory() {
			size, err = r.MemorySize()
			if err != nil && err != ErrDisposableSizeNotSupported {
				abort = true
			} else if size > 0 || err == ErrDisposableSizeNotSupported {
				dispose = true
			}
		}

		if abort {
			gc.resources = append(resources, gc.resources[i-1:]...)
			return err
		}

		if dispose {
			err = r.Dispose()
			if err != nil {
				if err != ErrDisposableInUse {
					gc.resources = append(resources, gc.resources[i-1:]...)
					return err
				}
				resources = append(resources, r)
			}
			continue
		}

		resources = append(resources, r)
	}

	gc.resources = resources
	return nil
}

// CollectAll disposes all resources that can be disposed.
//
// All resources not returning: ErrDisposableInUse.
// This is useful for testing when implementing resources.
func (gc *GarbageCollector) CollectAll() error {
	gc.m.Lock()
	defer gc.m.Unlock()
	var resources []Disposable
	for i, resource := range gc.resources {
		err := resource.Dispose()
		if err != nil {
			if err != ErrDisposableInUse {
				gc.resources = append(resources, gc.resources[i-1:]...)
				return err
			}
			resources = append(resources, resource)
		}
	}
	gc.resources = resources
	return nil
}

// needDiskSpace returns true if we need to free diskspace
func (gc *GarbageCollector) needDiskSpace() bool {
	// If we have no metrics or minimum diskspace we remove everything
	if gc.minimumDiskSpace == 0 || gc.storageFolder == "" {
		return true
	}
	stat, err := disk.Usage(gc.storageFolder)
	if err != nil {
		// TODO: Write a warning to the log
		return true
	}

	return int64(stat.Free) < gc.minimumDiskSpace
}

// needMemory returns true if we need to free memory
func (gc *GarbageCollector) needMemory() bool {
	// If we have no metrics or minimum memory we remove everything
	if gc.minimumMemory == 0 {
		return true
	}
	stat, err := mem.VirtualMemory()
	if err != nil {
		// TODO: Write a warning to the log
		return true
	}

	return int64(stat.Available) < gc.minimumMemory
}
