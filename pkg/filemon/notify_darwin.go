//go:build darwin
// +build darwin

package filemon

import "github.com/rjeczalik/notify"

func Watch(path string, c chan<- notify.EventInfo) error {
	return notify.Watch(path, c,
		notify.FSEventsIsFile,
		notify.FSEventsIsDir,
		notify.FSEventsChangeOwner,
		notify.All)
}
