//go:build windows

package winsys

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// HiddenProcAttr returns process attributes that keep a spawned console program
// from flashing a window on screen. Every mongod and probe this app runs uses it.
func HiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}

// IsElevated reports whether the current process holds administrator rights.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
