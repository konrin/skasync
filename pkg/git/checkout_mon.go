package git

import (
	"context"
	"errors"
	"os"
	"skasync/pkg/filemon"

	"github.com/rjeczalik/notify"
)

var (
	ErrGitHeadNotFound = errors.New("git head not found")
)

type CheckoutMon struct {
	rootDir     string
	currentHead string

	subscribes []func(filemon.ChangeList)
}

func NewCheckoutMon(rootDir string) *CheckoutMon {
	currentHead := readHead(rootDir)
	if len(currentHead) == 0 {
		panic(ErrGitHeadNotFound)
	}

	return &CheckoutMon{
		rootDir:     rootDir,
		currentHead: currentHead,
		subscribes:  make([]func(filemon.ChangeList), 0),
	}
}

func HasCheckoutMon(rootDir string) bool {
	if _, err := os.Stat(pathToHead(rootDir)); os.IsNotExist(err) {
		return false
	}

	return true
}

func (cm *CheckoutMon) Listen(ctx context.Context) error {
	c := make(chan notify.EventInfo, 100)
	if err := notify.Watch(pathToHead(cm.rootDir), c, notify.All); err != nil {
		return err
	}

	go func() { c <- nil }()

	for {
		select {
		case e := <-c:
			if e == nil {
				continue
			}

			yes, newHeadName, err := cm.isHeadChanged()
			if err != nil {
				return err
			}
			if !yes {
				continue
			}

			changeList, err := readDiffFilesChanged(cm.rootDir, cm.currentHead, newHeadName)
			if err != nil {
				return err
			}

			cm.currentHead = newHeadName

			go func() {
				for _, cb := range cm.subscribes {
					cb(changeList)
				}
			}()
		case <-ctx.Done():
			return nil
		}
	}
}

func (cm *CheckoutMon) isHeadChanged() (bool, string, error) {
	newHead := readHead(cm.rootDir)
	if len(newHead) == 0 {
		return false, "", ErrGitHeadNotFound
	}

	return cm.currentHead != newHead, newHead, nil
}

func (cm *CheckoutMon) Subscribe(cb func(filemon.ChangeList)) {
	cm.subscribes = append(cm.subscribes, cb)
}
