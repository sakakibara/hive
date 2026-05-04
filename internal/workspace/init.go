package workspace

import (
	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
)

// InitDir describes a directory to create during workspace initialization.
type InitDir struct {
	Label string
	Path  string
}

// InitLink describes a symlink to create during workspace initialization.
type InitLink struct {
	Label  string
	Link   string
	Target string
}

// ICloudDirs returns the directories needed for iCloud mode initialization.
func ICloudDirs(cfg *config.Config) []InitDir {
	resolved := cfg.Resolved()
	return []InitDir{
		{"iCloud workspace root", resolved.Storage.ICloudRoot},
		{"iCloud projects", cfg.ICloudDir("projects")},
		{"iCloud life", cfg.ICloudDir("life")},
		{"iCloud work", cfg.ICloudDir("work")},
		{"iCloud archive", cfg.ICloudDir("archive")},
		{"iCloud backups", cfg.ICloudDir("backups")},
		{"Local code root", resolved.Paths.Code},
	}
}

// ICloudLinks returns the symlinks needed for iCloud mode initialization.
func ICloudLinks(cfg *config.Config) []InitLink {
	resolved := cfg.Resolved()
	return []InitLink{
		{"~/Projects", resolved.Paths.Projects, cfg.ICloudDir("projects")},
		{"~/Life", resolved.Paths.Life, cfg.ICloudDir("life")},
		{"~/Work", resolved.Paths.Work, cfg.ICloudDir("work")},
		{"~/Archive", resolved.Paths.Archive, cfg.ICloudDir("archive")},
	}
}

// LocalDirs returns the directories needed for local mode initialization.
func LocalDirs(cfg *config.Config) []InitDir {
	resolved := cfg.Resolved()
	return []InitDir{
		{"Projects", resolved.Paths.Projects},
		{"Life", resolved.Paths.Life},
		{"Work", resolved.Paths.Work},
		{"Archive", resolved.Paths.Archive},
		{"Backups", cfg.BackupDir()},
		{"Code", resolved.Paths.Code},
	}
}

// EnsureDir creates a directory (and parents) if it does not exist.
func EnsureDir(path string) error {
	return fsutil.EnsureDir(path)
}

// EnsureSymlink creates a symlink at link pointing to target.
func EnsureSymlink(target, link string) error {
	return fsutil.EnsureSymlink(target, link)
}

// IsSymlink returns true if the path is a symlink.
func IsSymlink(path string) bool {
	return fsutil.IsSymlink(path)
}
