package util

import "sync"

type AverageStream struct {
	outCh chan int

	mu    sync.Mutex
	inMap map[string]int
}

func NewAverageStream(outCh chan int) *AverageStream {
	return &AverageStream{
		outCh: outCh,
		inMap: make(map[string]int),
	}
}

func (as *AverageStream) Set(id string, value int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.inMap[id] = value

	sum := 0
	for id := range as.inMap {
		sum += as.inMap[id]
	}

	as.outCh <- sum / len(as.inMap)
}
