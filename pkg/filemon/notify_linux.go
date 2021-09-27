//go:build linux
// +build linux

package filemon

import "github.com/rjeczalik/notify"

func Watch(path string, c chan<- notify.EventInfo) error {
	return notify.Watch(path, c, notify.All)
}
