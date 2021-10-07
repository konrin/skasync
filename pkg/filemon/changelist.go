package filemon

import (
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"
)

type ChangeList struct {
	mu       *sync.Mutex
	added    map[string]fs.FileInfo
	modified map[string]fs.FileInfo
	deleted  map[string]time.Time
}

func NewChangeList() ChangeList {
	return ChangeList{
		mu:       &sync.Mutex{},
		added:    make(map[string]fs.FileInfo),
		modified: make(map[string]fs.FileInfo),
		deleted:  make(map[string]time.Time),
	}
}

func (cl *ChangeList) Added() map[string]fs.FileInfo {
	list := make(map[string]fs.FileInfo)
	for path := range cl.added {
		list[path] = cl.added[path]
	}
	return list
}

func (cl *ChangeList) Modified() map[string]fs.FileInfo {
	list := make(map[string]fs.FileInfo)
	for path := range cl.modified {
		list[path] = cl.modified[path]
	}
	return list
}

func (cl *ChangeList) ModifiedAndAdded() map[string]fs.FileInfo {
	list := cl.Added()
	for path := range cl.Modified() {
		list[path] = cl.modified[path]
	}
	return list
}

func (cl *ChangeList) Deleted() map[string]time.Time {
	list := make(map[string]time.Time)
	for path := range cl.deleted {
		list[path] = cl.deleted[path]
	}
	return list
}

func (cl *ChangeList) AddAdded(filePath string, info fs.FileInfo) {
	cl.mu.Lock()
	cl.added[filePath] = info
	cl.mu.Unlock()
}

func (cl *ChangeList) RemoveAdded(filePath string) {
	cl.mu.Lock()
	delete(cl.added, filePath)
	cl.mu.Unlock()
}

func (cl *ChangeList) AddModified(filePath string, info fs.FileInfo) {
	cl.mu.Lock()
	cl.modified[filePath] = info
	cl.mu.Unlock()
}

func (cl *ChangeList) RemoveModified(filePath string) {
	cl.mu.Lock()
	delete(cl.modified, filePath)
	cl.mu.Unlock()
}

func (cl *ChangeList) AddDeleted(filePath string, t time.Time) {
	cl.mu.Lock()
	cl.deleted[filePath] = t
	cl.mu.Unlock()
}

func (cl *ChangeList) RemoveDeleted(filePath string) {
	cl.mu.Lock()
	delete(cl.deleted, filePath)
	cl.mu.Unlock()
}

func (cl *ChangeList) Union(list ChangeList) ChangeList {
	for filePath, fi := range list.added {
		cl.AddAdded(filePath, fi)
	}

	for filePath, fi := range list.modified {
		cl.AddModified(filePath, fi)
	}

	for filePath, t := range list.deleted {
		cl.AddDeleted(filePath, t)
	}

	return *cl
}

func (cl *ChangeList) CountAll() int {
	return len(cl.added) + len(cl.modified) + len(cl.deleted)
}

func (cl *ChangeList) HasDeletedFile(filePath string) bool {
	_, ok := cl.deleted[filePath]
	return ok
}

func (cl *ChangeList) HasModifiedFile(filePath string) bool {
	_, ok := cl.modified[filePath]
	return ok
}

func (cl *ChangeList) String(pref string) string {
	buf := ""
	buf += fmt.Sprintf("%sAdded (%d)\n", pref, len(cl.added))
	buf += fmt.Sprintf("%sModified (%d)\n", pref, len(cl.modified))
	buf += fmt.Sprintf("%sDeleted (%d)\n", pref, len(cl.deleted))

	return buf
}

func ChangeListUnion(lists []ChangeList) ChangeList {
	list := NewChangeList()

	for i := range lists {
		list = list.Union(lists[i])
	}

	return list
}

func ChangeFilesToChangeListConverter(files []string) ChangeList {
	list := NewChangeList()

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			if list.HasModifiedFile(filePath) {
				list.RemoveModified(filePath)
			}

			if !list.HasDeletedFile(filePath) {
				list.AddDeleted(filePath, time.Now())
			}

			continue
		}

		if info.IsDir() {
			continue
		}

		if list.HasDeletedFile(filePath) {
			list.RemoveDeleted(filePath)
		}

		if !list.HasModifiedFile(filePath) {
			list.AddModified(filePath, info)
		}
	}

	return list
}
