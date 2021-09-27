//go:build windows
// +build windows

package filemon

func Watch(path string, c chan<- notify.EventInfo) error {
	return notify.Watch(path, c, notify.All)
}
