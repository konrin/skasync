package filemon

import (
	"context"
	"log"
	"time"

	"github.com/rjeczalik/notify"
)

type Watcher struct {
	rootDir   string
	debaounce int
}

func NewWatcher(rootDir string, debaounce int) *Watcher {
	return &Watcher{
		rootDir:   rootDir,
		debaounce: debaounce,
	}
}

func (w *Watcher) Watch(ctx context.Context, outCh chan []string) error {
	c := make(chan notify.EventInfo, 100)

	if err := Watch(w.rootDir+"/...", c); err != nil {
		log.Fatal(err)
	}

	go func() { c <- nil }()

	changeFiles := make([]string, 0)

	timer := time.NewTimer(1<<63 - 1)
	for {
		select {
		case e := <-c:
			if e == nil {
				continue
			}

			changeFiles = append(changeFiles, e.Path())
			timer.Reset(time.Millisecond * time.Duration(w.debaounce))
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
