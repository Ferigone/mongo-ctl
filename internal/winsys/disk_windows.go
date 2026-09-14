//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// FreeDiskSpace returns the bytes available to the current user on the volume
// holding path. The nearest existing ancestor is measured, so a directory that
// has not been created yet can still be checked.
func FreeDiskSpace(path string) (uint64, error) {
	existing, err := nearestExistingDir(path)
	if err != nil {
		return 0, err
	}

	pathPtr, err := windows.UTF16PtrFromString(existing)
	if err != nil {
		return 0, fmt.Errorf("invalid path %q: %w", existing, err)
	}

	var freeToCaller, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeToCaller, &totalBytes, &totalFree); err != nil {
		return 0, fmt.Errorf("query free space for %q: %w", existing, err)
	}
	return freeToCaller, nil
}

func nearestExistingDir(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}

	for {
		if _, err := windows.UTF16PtrFromString(current); err != nil {
			return "", fmt.Errorf("invalid path %q: %w", current, err)
		}
		if dirExists(current) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return current, nil
		}
		current = parent
	}
}
