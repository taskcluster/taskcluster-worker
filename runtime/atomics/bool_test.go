package atomics

import (
	"fmt"
	"sync"
	t "testing"
	"time"
)

func trueOrPanic(condition bool, a ...interface{}) {
	if !condition {
		panic(fmt.Sprintln(a...))
	}
}

func TestBoolZeroValue(t *t.T) {
	b := Bool{}
	trueOrPanic(!b.Get(), "Expected zero-value to be false")
}

func TestNewBool(t *t.T) {
	b := NewBool(false)
	trueOrPanic(!b.Get(), "Expected false")
	b = NewBool(true)
	trueOrPanic(b.Get(), "Expected true")
}

func TestBoolSet(t *t.T) {
	b := Bool{}
	b.Set(false)
	trueOrPanic(!b.Get(), "Expected Set(false) to set it false")
	b.Set(true)
	trueOrPanic(b.Get(), "Expected Set(true) to set it true")
	b.Set(false)
	trueOrPanic(!b.Get(), "Expected Set(false) to set it false")
	b.Set(true)
	trueOrPanic(b.Get(), "Expected Set(true) to set it true")
}

func TestBoolSwap(t *t.T) {
	b := Bool{}
	trueOrPanic(!b.Swap(false), "Expected zero-value from swap as false")
	trueOrPanic(!b.Swap(true), "Expected false from swap")
	trueOrPanic(b.Get(), "Expected Swap(true) to leave it true")
	trueOrPanic(b.Swap(true), "Expected true from swap")
	trueOrPanic(b.Swap(false), "Expected true from swap as swapped")
	trueOrPanic(!b.Get(), "Expected Swap(false) to leave it false")
}

func TestBoolInSpinLock(t *t.T) {
	b := Bool{}
	wg := sync.WaitGroup{}
	wg.Add(2)
	// Okay, we're really just doing weird things here assuming that the
	// race condition detector will tell us about any issues.
	go func() {
		for !b.Get() {
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(7 * time.Millisecond)
		b.Set(false)
		wg.Done()
	}()
	go func() {
		time.Sleep(15 * time.Millisecond)
		b.Set(true)
		for b.Swap(true) {
			time.Sleep(5 * time.Millisecond)
		}
		wg.Done()
	}()
	wg.Wait()
}
