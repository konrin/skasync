package util

import "sync"

type SafeCounter struct {
	mu    sync.Mutex
	count int
}

func (sc *SafeCounter) Inc() {
	sc.mu.Lock()
	sc.count++
	sc.mu.Unlock()
}

func (sc *SafeCounter) Add(val int) {
	sc.mu.Lock()
	sc.count += val
	sc.mu.Unlock()
}

func (sc *SafeCounter) Value() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.count
}
