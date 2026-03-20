package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir creates a directory (and parents) if it does not exist.
// Returns an error if the path exists but is not a directory.
func EnsureDir(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("%s exists but is not a directory", path)
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(path, 0755)
}

// EnsureSymlink creates a symlink at linkPath pointing to target.
// If the symlink already exists and points to the correct target, it is a no-op.
// Returns an error if linkPath exists but is not a symlink or points elsewhere.
func EnsureSymlink(target, linkPath string) error {
	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s exists but is not a symlink (is %s)", linkPath, fileTypeDesc(info))
		}
		existing, err := os.Readlink(linkPath)
		if err != nil {
			return fmt.Errorf("read symlink %s: %w", linkPath, err)
		}
		if existing == target {
			return nil
		}
		return fmt.Errorf("%s is a symlink but points to %s (expected %s)", linkPath, existing, target)
	}
	if !os.IsNotExist(err) {
		return err
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return err
	}

	return os.Symlink(target, linkPath)
}

// PathExists returns true if the path exists (follows symlinks).
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsSymlink returns true if the path is a symlink.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// IsDir returns true if the path is a directory (follows symlinks).
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// SymlinkTarget returns the target of a symlink, or empty string if not a symlink.
func SymlinkTarget(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

// PathConflict checks if a path exists and is not the expected type.
// Returns a human-readable error message, or empty string if the path is free.
func PathConflict(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return fmt.Sprintf("cannot stat %s: %v", path, err)
	}
	return fmt.Sprintf("%s already exists (%s)", path, fileTypeDesc(info))
}

func fileTypeDesc(info os.FileInfo) string {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.IsDir():
		return "directory"
	default:
		return "regular file"
	}
}
