package atomics

import (
	"sync"
	"testing"
)

func TestOnceDoTwice(t *testing.T) {
	var once Once
	count := 0
	once.Do(func() {
		count++
	})
	once.Wait()
	once.Do(func() {
		count++
	})
	once.Wait()
	if count != 1 {
		panic("Expected count == 1")
	}
}

func TestOnceDoConcurrent(t *testing.T) {
	var once Once
	mCount := sync.Mutex{}
	count := 0
	mRCount := sync.Mutex{}
	rCount := 0
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			go func() {
				once.Wait()
				mCount.Lock()
				if count != 1 {
					panic("Expected count == 1, after once.Wait()")
				}
				mCount.Unlock()
			}()
			result := once.Do(func() {
				mCount.Lock()
				count++
				mCount.Unlock()
			})
			if result {
				mRCount.Lock()
				rCount++
				mRCount.Unlock()
			}
			wg.Done()
		}()
	}

	wg.Wait()
	if count != 1 {
		panic("Expected count == 1")
	}
	if rCount != 1 {
		panic("Expected rCount == 1")
	}
}

func TestOnceDoConcurrent2(t *testing.T) { // Just moving code blocks
	var once Once
	mCount := sync.Mutex{}
	count := 0
	mRCount := sync.Mutex{}
	rCount := 0
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			result := once.Do(func() {
				mCount.Lock()
				count++
				mCount.Unlock()
			})
			if result {
				mRCount.Lock()
				rCount++
				mRCount.Unlock()
			}
			wg.Done()
		}()
	}
	go func() {
		once.Wait()
		mCount.Lock()
		if count != 1 {
			panic("Expected count == 1, after once.Wait()")
		}
		mCount.Unlock()
	}()

	wg.Wait()
	if count != 1 {
		panic("Expected count == 1")
	}
	if rCount != 1 {
		panic("Expected rCount == 1")
	}
}

func TestOnceDoConcurrent3(t *testing.T) { // Just moving code blocks
	var once Once
	mCount := sync.Mutex{}
	count := 0
	mRCount := sync.Mutex{}
	rCount := 0
	wg := sync.WaitGroup{}
	go func() {
		once.Wait()
		mCount.Lock()
		if count != 1 {
			panic("Expected count == 1, after once.Wait()")
		}
		mCount.Unlock()
	}()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			result := once.Do(func() {
				mCount.Lock()
				count++
				mCount.Unlock()
			})
			if result {
				mRCount.Lock()
				rCount++
				mRCount.Unlock()
			}
			wg.Done()
		}()
	}

	wg.Wait()
	if count != 1 {
		panic("Expected count == 1")
	}
	if rCount != 1 {
		panic("Expected rCount == 1")
	}
}

func TestOnceDoNestedDo(t *testing.T) {
	var once Once
	count := 0
	once.Do(func() {
		count++
		once.Do(func() {
			count++
			panic("this shouldn't happen")
		})
	})
	once.Wait()
	if count != 1 {
		panic("Expected count == 1")
	}
}
