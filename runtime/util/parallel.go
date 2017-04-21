package util

import "sync"

// Parallel takes a list of functions and calls them all in concurrently,
// returning when all the functions are done.
//
// This doesn't have any nice error or panic handling and is aimed as construct
// to be used inside other functions, mainly to reduce boiler-plate.
func Parallel(f ...func()) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	for _, fn := range f {
		wg.Add(1)
		go func(fn func()) {
			fn()
			wg.Done()
		}(fn)
	}

	wg.Done()
	wg.Wait()
}

// Spawn N go routines and wait for them to return.
//
// This utility is smart when instantiating elements in an array concurrently.
func Spawn(N int, fn func(i int)) {
	wg := sync.WaitGroup{}
	wg.Add(N)
	for index := 0; index < N; index++ {
		go func(i int) {
			defer wg.Done()
			fn(i)
		}(index)
	}
	wg.Wait()
}
