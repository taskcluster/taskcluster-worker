package gc

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func assert(cond bool, a ...interface{}) {
	if !cond {
		panic(fmt.Sprint(a...))
	}
}

type myResource struct {
	DisposableResource
	disposed bool
}

func (r *myResource) Dispose() error {
	if err := r.CanDispose(); err != nil {
		return err
	}
	if r.disposed {
		panic("Already disposed once!")
	}
	r.disposed = true
	return nil
}

func TestGarbageCollector(t *testing.T) {
	fmt.Println(" - Create GC")
	gc := &GarbageCollector{}
	err := gc.CollectAll()
	assert(err == nil, "Didn't expect error: ", err)

	fmt.Println(" - Create r1")
	r1 := &myResource{}
	gc.Register(r1)
	assert(r1.disposed == false, "Not disposed")

	fmt.Println(" - CollectAll() disposing r1")
	gc.CollectAll()
	assert(r1.disposed, "Expected it to be disposed")

	fmt.Println(" - Create r2 and Acquire")
	r2 := &myResource{}
	r2.Acquire()
	gc.Register(r2)
	assert(r2.disposed == false, "Not disposed")

	fmt.Println(" - CollectAll() disposing nothing")
	gc.CollectAll()
	assert(r2.disposed == false, "Not disposed")

	fmt.Println(" - Release and CollectAll()")
	r2.Release()
	assert(r2.disposed == false, "Not disposed")
	gc.CollectAll()
	assert(r2.disposed, "disposed")

	fmt.Println(" - Create r3 and Acquire")
	r3 := &myResource{}
	r3.Acquire()
	gc.Register(r3)
	assert(r3.disposed == false, "Not disposed")

	fmt.Println(" - CollectAll() getting nothing")
	gc.CollectAll()
	assert(r3.disposed == false, "Not disposed")

	fmt.Println(" - Acquire() and CollectAll() getting nothing")
	r3.Acquire()
	gc.CollectAll()
	assert(r3.disposed == false, "Not disposed")

	fmt.Println(" - Release() and CollectAll() getting nothing")
	r3.Release()
	gc.CollectAll()
	assert(r3.disposed == false, "Not disposed")

	fmt.Println(" - Release and CollectAll() getting r3")
	r3.Release()
	assert(r3.disposed == false, "Not disposed")
	gc.CollectAll()
	assert(r3.disposed, "disposed")
}

type testResource struct {
	mem          uint64
	disk         uint64
	diskError    error
	memError     error
	disposed     bool
	disposeError error
	lastUsed     time.Time
}

func (t *testResource) MemorySize() (uint64, error) {
	return t.mem, t.memError
}
func (t *testResource) DiskSize() (uint64, error) {
	return t.disk, t.diskError
}
func (t *testResource) LastUsed() time.Time {
	return t.lastUsed
}
func (t *testResource) Dispose() error {
	if t.disposeError == nil {
		t.disposed = true
	}
	return t.disposeError
}

func TestCollectDiskOnly(t *testing.T) {
	gc := &GarbageCollector{
		storageFolder:    "...",
		minimumDiskSpace: math.MaxInt64,
		minimumMemory:    1,
	}

	// Add two resources only r1 should be disposed
	r1 := &testResource{
		mem:      0,
		disk:     10,
		lastUsed: time.Now(),
	}
	gc.Register(r1)
	r2 := &testResource{
		mem:      10,
		disk:     0,
		lastUsed: time.Now(),
	}
	gc.Register(r2)

	gc.Collect()
	assert(r1.disposed, "Expected r1 to be disposed")
	assert(!r2.disposed, "Didn't expect r2 to be disposed")
}

func TestCollectDiskOnlyInUse(t *testing.T) {
	gc := &GarbageCollector{
		storageFolder:    "...",
		minimumDiskSpace: math.MaxInt64,
		minimumMemory:    1,
	}

	// Add two resources neither should be disposed
	r1 := &testResource{
		mem:          0,
		disk:         10,
		lastUsed:     time.Now(),
		disposeError: ErrDisposableInUse,
	}
	gc.Register(r1)
	r2 := &testResource{
		mem:      10,
		disk:     0,
		lastUsed: time.Now(),
	}
	gc.Register(r2)

	gc.Collect()
	assert(!r1.disposed, "Didn't expect r1 to be disposed")
	assert(!r2.disposed, "Didn't expect r2 to be disposed")
}

func TestCollectMemoryOnly(t *testing.T) {
	gc := &GarbageCollector{
		storageFolder:    os.TempDir(),
		minimumDiskSpace: 1,
		minimumMemory:    math.MaxInt64,
	}

	// Add two resources only r1 should be disposed
	r1 := &testResource{
		mem:      10,
		disk:     0,
		lastUsed: time.Now(),
	}
	gc.Register(r1)
	r2 := &testResource{
		mem:      0,
		disk:     10,
		lastUsed: time.Now(),
	}
	gc.Register(r2)

	gc.Collect()
	assert(r1.disposed, "Expected r1 to be disposed")
	assert(!r2.disposed, "Didn't expect r2 to be disposed")
}
