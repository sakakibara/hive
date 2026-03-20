package fsutil

import (
	"fmt"
	"io"
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

// MoveDir moves src to dst, falling back to recursive copy + remove for cross-device moves.
func MoveDir(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	// os.Rename fails across devices; fall back to copy + remove.
	if err := copyDirRecursive(src, dst); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	return os.RemoveAll(src)
}

func copyDirRecursive(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Use Lstat to detect symlinks.
		info, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, dstPath); err != nil {
				return err
			}
		case info.IsDir():
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		default:
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// IsEmptyDir returns true if the path is an empty directory.
func IsEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}

// CleanEmptyParents removes empty parent directories up to (but not including) the stop directory.
func CleanEmptyParents(path, stopAt string) {
	for {
		parent := filepath.Dir(path)
		if parent == stopAt || parent == path || !IsEmptyDir(parent) {
			break
		}
		os.Remove(parent)
		path = parent
	}
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
