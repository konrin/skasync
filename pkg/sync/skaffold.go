package sync

import (
	"context"
	"fmt"
	"skasync/pkg/k8s"
	"skasync/pkg/skaffold"
	"sync"
)

type SkaffoldStatusLayer struct {
	isWatching       bool
	outChangeFilesCh chan []string

	podCtrl *k8s.PodsCtrl

	mu                sync.Mutex
	lastStatus        skaffold.SkaffoldProcessStatus
	changeFilesBuffer map[string]struct{}
}

func NewSkaffoldStatusLayer(isWatching bool, outChangeFilesCh chan []string, podCtrl *k8s.PodsCtrl) *SkaffoldStatusLayer {
	return &SkaffoldStatusLayer{
		isWatching:        isWatching,
		podCtrl:           podCtrl,
		outChangeFilesCh:  outChangeFilesCh,
		changeFilesBuffer: make(map[string]struct{}),
	}
}

func (ssl *SkaffoldStatusLayer) StatusHandler(status skaffold.SkaffoldProcessStatus) {
	if !ssl.isWatching {
		return
	}

	ssl.mu.Lock()
	defer ssl.mu.Unlock()

	if ssl.lastStatus.IsReady && !status.IsReady {
		println("Skaffold deploy is down")
	}

	if !ssl.lastStatus.IsReady && status.IsReady {
		println("Skaffold deploy is up")
		err := ssl.podCtrl.Refresh()
		if err != nil {
			panic(err)
		}

		println("Watching for changes...")

		if ssl.bufferCount() > 0 {
			fmt.Printf("Sync change files from buffer (%d)\n", ssl.bufferCount())

			ssl.outChangeFilesCh <- ssl.getChangeFilesBuffer()
			ssl.cleanBuffer()
		}
	}

	ssl.lastStatus = status
}

func (ssl *SkaffoldStatusLayer) Do(ctx context.Context, inChangeFilesCh chan []string) error {
	for {
		select {
		case changeFiles := <-inChangeFilesCh:
			if !ssl.isWatching || ssl.lastStatus.IsReady {
				ssl.outChangeFilesCh <- changeFiles
				break
			}

			ssl.mu.Lock()
			ssl.appendToBuffer(changeFiles)
			ssl.mu.Unlock()

			if ssl.lastStatus.DoesNotAnswer {
				fmt.Printf("Skaffold is down, awaiting start... (%d) files in buffer\n", len(ssl.changeFilesBuffer))
			} else if !ssl.lastStatus.IsReady {
				fmt.Printf("Awaiting deploy... (%d) files in buffer\n", len(ssl.changeFilesBuffer))
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (ssl *SkaffoldStatusLayer) appendToBuffer(files []string) {
	for _, filePath := range files {
		ssl.changeFilesBuffer[filePath] = struct{}{}
	}
}

func (ssl *SkaffoldStatusLayer) getChangeFilesBuffer() []string {
	buf := make([]string, 0, len(ssl.changeFilesBuffer))

	for filePath := range ssl.changeFilesBuffer {
		buf = append(buf, filePath)
	}

	return buf
}

func (ssl *SkaffoldStatusLayer) bufferCount() int {
	return len(ssl.changeFilesBuffer)
}

func (ssl *SkaffoldStatusLayer) cleanBuffer() {
	ssl.changeFilesBuffer = make(map[string]struct{})
}
