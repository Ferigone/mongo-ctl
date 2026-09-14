package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mongoctl/internal/winsys"
)

// moveDirectory relocates a data directory. os.Rename handles the common case;
// moving to another volume needs a copy, because Windows cannot rename across them.
func moveDirectory(source, target string) error {
	targetExisted, err := prepareTarget(target)
	if err != nil {
		return err
	}

	if err := os.Rename(source, target); err == nil {
		return nil
	}

	if err := ensureFreeSpace(source, target); err != nil {
		return err
	}

	if err := copyTree(source, target); err != nil {
		if !targetExisted {
			os.RemoveAll(target)
		}
		return err
	}
	if err := os.RemoveAll(source); err != nil {
		return fmt.Errorf("remove original data directory %q after copy: %w", source, err)
	}
	return nil
}

// prepareTarget validates the destination and reports whether it already existed.
func prepareTarget(target string) (bool, error) {
	info, err := os.Stat(target)
	switch {
	case err == nil:
		if !info.IsDir() {
			return false, fmt.Errorf("target %q is not a directory", target)
		}
		empty, err := directoryEmpty(target)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, fmt.Errorf("target directory %q is not empty", target)
		}
		return true, nil

	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return false, fmt.Errorf("create parent of %q: %w", target, err)
		}
		return false, nil

	default:
		return false, fmt.Errorf("inspect target %q: %w", target, err)
	}
}

func ensureFreeSpace(source, target string) error {
	needed, err := directorySize(source)
	if err != nil {
		return err
	}
	available, err := winsys.FreeDiskSpace(target)
	if err != nil {
		return err
	}
	if available < needed {
		return fmt.Errorf(
			"not enough free space at the target: need %d bytes, %d available",
			needed, available,
		)
	}
	return nil
}

func directoryEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read directory %q: %w", dir, err)
	}
	return len(entries) == 0, nil
}

func directorySize(dir string) (uint64, error) {
	var total uint64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += uint64(info.Size())
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("measure directory %q: %w", dir, err)
	}
	return total, nil
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)

		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %q: %w", source, err)
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create %q: %w", destination, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %q: %w", source, err)
	}
	return out.Sync()
}
