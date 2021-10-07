package filemon

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rjeczalik/notify"
)

type Watcher struct {
	rootDir   string
	debounce int
}

func NewWatcher(rootDir string, debounce int) *Watcher {
	return &Watcher{
		rootDir:   rootDir,
		debounce: debounce,
	}
}

func (w *Watcher) Watch(ctx context.Context, outCh chan []string) error {
	c := make(chan notify.EventInfo, 100)

	if err := Watch(w.rootDir+"/...", c); err != nil {
		log.Fatal(err)
	}

	go func() { c <- nil }()

	changeFiles := make([]string, 0)

	d := time.Duration(0)
	if (w.debounce > 0) {
		d = time.Millisecond * time.Duration(w.debounce / 2)
	}

	timer := time.NewTimer(1<<63 - 1)
	for {
		select {
		case e := <-c:
			if e == nil {
				continue
			}

			changeFiles = append(changeFiles, e.Path())
			timer.Reset(d)
		case <-timer.C:
			// send change list
			outCh <- changeFiles
			changeFiles = make([]string, 0)
		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}

func ConvertFilesToChangeList(files []string) ChangeList {
	list := NewChangeList()

	for _, filePath := range files {
		fi, err := os.Stat(filePath)
		if fi.IsDir() {
			continue
		}

		if os.IsExist(err) {
			list.AddModified(filePath, fi)
			continue
		}

		list.AddDeleted(filePath, time.Now())
	}

	return list
}
