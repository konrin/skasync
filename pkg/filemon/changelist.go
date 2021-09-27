package filemon

import (
	"fmt"
	"io/fs"
	"os"
	"time"
)

type ChangeList struct {
	Modified map[string]fs.FileInfo
	Deleted  map[string]time.Time
}

func NewChangeList() ChangeList {
	return ChangeList{
		Modified: make(map[string]fs.FileInfo),
		Deleted:  make(map[string]time.Time),
	}
}

func (cl *ChangeList) addModified(filePath string, info fs.FileInfo) {
	cl.Modified[filePath] = info
}

func (cl *ChangeList) removeModified(filePath string) {
	delete(cl.Modified, filePath)
}

func (cl *ChangeList) addDeleted(filePath string, t time.Time) {
	cl.Deleted[filePath] = t
}

func (cl *ChangeList) removeDeleted(filePath string) {
	delete(cl.Deleted, filePath)
}

func (cl *ChangeList) HasDeletedFile(filePath string) bool {
	_, ok := cl.Deleted[filePath]
	return ok
}

func (cl *ChangeList) HasModifiedFile(filePath string) bool {
	_, ok := cl.Modified[filePath]
	return ok
}

func (cl *ChangeList) String() string {
	buf := ""

	buf += fmt.Sprintf("Deleted (%d) ---\n", len(cl.Deleted))
	for key := range cl.Deleted {
		buf += fmt.Sprintf("	- %s\n", key)
	}
	buf += fmt.Sprintf("Modified (%d) ~~~\n", len(cl.Modified))
	for key := range cl.Modified {
		buf += fmt.Sprintf("	- %s\n", key)
	}

	return buf
}

// func FilesMapDiff(filesMap1 filesystem.FilesMap, filesMap2 filesystem.FilesMap) ChangeList {
// 	return NewChangeList()
// }

// type ChangeListCtrl struct {
// 	rootDir string
// 	list    *ChangeList
// 	startAt time.Time
// }

// func NewChangeListCtrl(rootDir string) *ChangeListCtrl {
// 	list := NewChangeList()
// 	return &ChangeListCtrl{
// 		rootDir: rootDir,
// 		list:    &list,
// 		startAt: time.Now(),
// 	}
// }

// func (cl *ChangeListCtrl) Do(filePath string) {
// 	info, err := os.Stat(filePath)
// 	if os.IsNotExist(err) {
// 		if cl.list.HasModifiedFile(filePath) {
// 			cl.list.removeModified(filePath)
// 		}

// 		cl.list.addDeleted(filePath, time.Now())

// 		return
// 	}

// 	if info.IsDir() {
// 		return
// 	}

// 	if cl.list.HasDeletedFile(filePath) {
// 		cl.list.removeDeleted(filePath)
// 	}

// 	cl.list.addModified(filePath, info)
// }

// func (cl *ChangeListCtrl) Reset() {
// 	cl.list.reset()
// 	cl.startAt = time.Now()
// }

// func (cl *ChangeListCtrl) GetChangeList() *ChangeList {
// 	return cl.list
// }

func ChangeFilesToChangeListConverter(files []string) ChangeList {
	list := NewChangeList()

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			if list.HasModifiedFile(filePath) {
				list.removeModified(filePath)
			}

			if !list.HasDeletedFile(filePath) {
				list.addDeleted(filePath, time.Now())
			}

			continue
		}

		if info.IsDir() {
			continue
		}

		if list.HasDeletedFile(filePath) {
			list.removeDeleted(filePath)
		}

		if !list.HasModifiedFile(filePath) {
			list.addModified(filePath, info)
		}
	}

	return list
}
