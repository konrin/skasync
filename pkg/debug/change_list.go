package debug

import (
	"skasync/pkg/filemon"
	"sync"
)

type ChangeList struct {
	mu   sync.Mutex
	c    int
	list map[int]map[string]filemon.ChangeList
}

func NewChangeList() *ChangeList {
	return &ChangeList{
		list: make(map[int]map[string]filemon.ChangeList),
	}
}

func (cl *ChangeList) AddResult(r map[string]filemon.ChangeList) int {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.c++

	cl.list[cl.c] = r

	return cl.c
}

func (cl *ChangeList) Get(id int) map[string]filemon.ChangeList {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	list, ok := cl.list[id]
	if !ok {
		return nil
	}

	return list
}
