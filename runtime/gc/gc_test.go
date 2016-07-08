package gc

import "testing"
import "fmt"

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
