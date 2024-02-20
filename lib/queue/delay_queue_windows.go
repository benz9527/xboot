//go:build windows
// +build windows

package queue

import "golang.org/x/sys/windows"

func init() {
	if err := windows.TimeBeginPeriod(1); err != nil {
		panic(err)
	}
	if err := windows.TimeEndPeriod(1); err != nil {
		panic(err)
	}
}
