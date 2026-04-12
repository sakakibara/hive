package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/meta"
)

func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

// DiagLevel represents the severity of a diagnostic result.
type DiagLevel int

const (
	DiagOK   DiagLevel = iota
	DiagInfo DiagLevel = iota
	DiagWarn DiagLevel = iota
	DiagErr  DiagLevel = iota
)

// String returns a human-readable label for the diagnostic level.
func (l DiagLevel) String() string {
	switch l {
	case DiagOK:
		return "OK"
	case DiagInfo:
		return "INFO"
	case DiagWarn:
		return "WARN"
	case DiagErr:
		return "ERROR"
	default:
		return "?"
	}
}

// DiagResult is a single diagnostic finding.
type DiagResult struct {
	Level   DiagLevel
	Message string
}

// DiagReport collects all diagnostic results from a doctor run.
type DiagReport struct {
	Results []DiagResult
}

func (r *DiagReport) add(level DiagLevel, format string, args ...any) {
	r.Results = append(r.Results, DiagResult{
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *DiagReport) ok(format string, args ...any)   { r.add(DiagOK, format, args...) }
func (r *DiagReport) info(format string, args ...any)  { r.add(DiagInfo, format, args...) }
func (r *DiagReport) warn(format string, args ...any)  { r.add(DiagWarn, format, args...) }
func (r *DiagReport) err(format string, args ...any)   { r.add(DiagErr, format, args...) }

// HasErrors returns true if any result has error level.
func (r *DiagReport) HasErrors() bool {
	for _, res := range r.Results {
		if res.Level == DiagErr {
			return true
		}
	}
	return false
}

// Doctor runs workspace health checks and returns a diagnostic report.
func Doctor(cfg *config.Config) *DiagReport {
	r := &DiagReport{}
	resolved := cfg.Resolved()

	if cfg.IsICloud() {
		checkICloudMode(r, cfg, resolved)
	} else {
		checkLocalMode(r, cfg, resolved)
	}

	checkProjects(r, cfg)

	return r
}

func checkICloudMode(r *DiagReport, cfg *config.Config, resolved *config.Config) {
	checkDir(r, "iCloud workspace root", resolved.Storage.ICloudRoot)
	checkDir(r, "iCloud projects dir", cfg.ICloudDir("projects"))
	checkDir(r, "iCloud life dir", cfg.ICloudDir("life"))
	checkDir(r, "iCloud work dir", cfg.ICloudDir("work"))
	checkDir(r, "iCloud archive dir", cfg.ICloudDir("archive"))
	checkDir(r, "iCloud backups dir", cfg.ICloudDir("backups"))
	checkDir(r, "Code root", resolved.Paths.Code)

	checkSymlinkHealth(r, "~/Projects", resolved.Paths.Projects, cfg.ICloudDir("projects"))
	checkSymlinkHealth(r, "~/Life", resolved.Paths.Life, cfg.ICloudDir("life"))
	checkSymlinkHealth(r, "~/Work", resolved.Paths.Work, cfg.ICloudDir("work"))
	checkSymlinkHealth(r, "~/Archive", resolved.Paths.Archive, cfg.ICloudDir("archive"))
}

func checkLocalMode(r *DiagReport, cfg *config.Config, resolved *config.Config) {
	checkRealDir(r, "Projects", resolved.Paths.Projects)
	checkRealDir(r, "Life", resolved.Paths.Life)
	checkRealDir(r, "Work", resolved.Paths.Work)
	checkRealDir(r, "Archive", resolved.Paths.Archive)
	checkRealDir(r, "Backups", cfg.BackupDir())
	checkRealDir(r, "Code", resolved.Paths.Code)
}

func checkDir(r *DiagReport, label, path string) {
	if fsutil.IsDir(path) {
		r.ok("%s (%s)", label, tildePath(path))
	} else {
		r.err("%s missing or not a directory: %s", label, tildePath(path))
	}
}

func checkRealDir(r *DiagReport, label, path string) {
	if fsutil.IsSymlink(path) {
		r.err("%s (%s) is a symlink — local mode requires real directories", label, tildePath(path))
		return
	}
	if fsutil.IsDir(path) {
		r.ok("%s (%s)", label, tildePath(path))
	} else {
		r.err("%s missing or not a directory: %s", label, tildePath(path))
	}
}

func checkSymlinkHealth(r *DiagReport, label, linkPath, expectedTarget string) {
	displayLink := tildePath(linkPath)
	displayTarget := tildePath(expectedTarget)
	if !fsutil.IsSymlink(linkPath) {
		r.err("%s is not a symlink (expected → %s)", displayLink, displayTarget)
		return
	}
	target := fsutil.SymlinkTarget(linkPath)
	if target != expectedTarget {
		r.err("%s → %s (expected %s)", displayLink, tildePath(target), displayTarget)
		return
	}
	r.ok("%s → %s", displayLink, displayTarget)
}

func checkProjects(r *DiagReport, cfg *config.Config) {
	resolved := cfg.Resolved()
	projectsDir, err := filepath.EvalSymlinks(resolved.Paths.Projects)
	if err != nil {
		return
	}

	if !fsutil.IsDir(projectsDir) {
		return
	}

	_ = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.Name() != meta.FileName {
			return nil
		}

		m, readErr := meta.Read(path)
		if readErr != nil {
			r.err("Invalid metadata: %s (%v)", tildePath(path), readErr)
			return nil
		}

		issues := m.Validate()
		for _, issue := range issues {
			r.err("%s: %s", m.Name, issue)
		}

		if !m.HasCode() {
			r.ok("%s (%s): no code", m.Name, m.Org)
			return nil
		}

		projRoot := filepath.Dir(path)
		codeLinkPath := filepath.Join(projRoot, "code")
		expectedCodeTarget := filepath.Join(resolved.Paths.Code, m.CodeRel)

		if !fsutil.IsSymlink(codeLinkPath) {
			r.err("%s: code symlink missing at %s", m.Name, tildePath(codeLinkPath))
		} else {
			target := fsutil.SymlinkTarget(codeLinkPath)
			if target != expectedCodeTarget {
				r.err("%s: code symlink → %s (expected %s)", m.Name, tildePath(target), tildePath(expectedCodeTarget))
			} else if !fsutil.PathExists(target) {
				r.err("%s: code symlink target does not exist: %s", m.Name, tildePath(target))
			} else {
				r.ok("%s: code → %s", m.Name, tildePath(target))
			}
		}

		// Check tracked repos.
		for repoName, repoURL := range m.Repos {
			repoPath := filepath.Join(expectedCodeTarget, repoName)
			if fsutil.PathExists(repoPath) {
				r.ok("%s: repo %s", m.Name, repoName)
			} else if repoURL != "" {
				r.err("%s: repo %s missing (url: %s)", m.Name, repoName, repoURL)
			} else {
				r.err("%s: repo %s missing", m.Name, repoName)
			}
		}

		// Detect repos on disk that are not tracked in metadata.
		if fsutil.IsDir(expectedCodeTarget) {
			entries, dirErr := os.ReadDir(expectedCodeTarget)
			if dirErr == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					name := entry.Name()
					if strings.HasPrefix(name, ".") {
						continue
					}
					if _, tracked := m.Repos[name]; !tracked {
						r.warn("%s: repo %s exists on disk but is not tracked in metadata", m.Name, name)
					}
				}
			}
		}

		if len(m.Repos) == 0 {
			r.info("%s: no repos tracked", m.Name)
		}

		return nil
	})
}
