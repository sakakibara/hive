package doctor

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

type Level int

const (
	LevelOK   Level = iota
	LevelInfo Level = iota
	LevelWarn Level = iota
	LevelErr  Level = iota
)

func (l Level) String() string {
	switch l {
	case LevelOK:
		return "OK"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelErr:
		return "ERROR"
	default:
		return "?"
	}
}

type Result struct {
	Level   Level
	Message string
}

type Report struct {
	Results []Result
}

func (r *Report) add(level Level, format string, args ...any) {
	r.Results = append(r.Results, Result{
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}

func (r *Report) ok(format string, args ...any)   { r.add(LevelOK, format, args...) }
func (r *Report) info(format string, args ...any)  { r.add(LevelInfo, format, args...) }
func (r *Report) warn(format string, args ...any)  { r.add(LevelWarn, format, args...) }
func (r *Report) err(format string, args ...any)   { r.add(LevelErr, format, args...) }

func (r *Report) HasErrors() bool {
	for _, res := range r.Results {
		if res.Level == LevelErr {
			return true
		}
	}
	return false
}

func Run(cfg *config.Config) *Report {
	r := &Report{}
	resolved := cfg.Resolved()

	if cfg.IsICloud() {
		checkICloudMode(r, cfg, resolved)
	} else {
		checkLocalMode(r, resolved)
	}

	checkProjects(r, cfg)

	return r
}

func checkICloudMode(r *Report, cfg *config.Config, resolved *config.Config) {
	checkDir(r, "iCloud workspace root", resolved.Storage.ICloudRoot)
	checkDir(r, "iCloud projects dir", cfg.ICloudDir("projects"))
	checkDir(r, "iCloud life dir", cfg.ICloudDir("life"))
	checkDir(r, "iCloud work dir", cfg.ICloudDir("work"))
	checkDir(r, "iCloud archive dir", cfg.ICloudDir("archive"))
	checkDir(r, "Code root", resolved.Paths.Code)

	checkSymlink(r, "~/Projects", resolved.Paths.Projects, cfg.ICloudDir("projects"))
	checkSymlink(r, "~/Life", resolved.Paths.Life, cfg.ICloudDir("life"))
	checkSymlink(r, "~/Work", resolved.Paths.Work, cfg.ICloudDir("work"))
	checkSymlink(r, "~/Archive", resolved.Paths.Archive, cfg.ICloudDir("archive"))
}

func checkLocalMode(r *Report, resolved *config.Config) {
	checkRealDir(r, "Projects", resolved.Paths.Projects)
	checkRealDir(r, "Life", resolved.Paths.Life)
	checkRealDir(r, "Work", resolved.Paths.Work)
	checkRealDir(r, "Archive", resolved.Paths.Archive)
	checkRealDir(r, "Code", resolved.Paths.Code)
}

func checkDir(r *Report, label, path string) {
	if fsutil.IsDir(path) {
		r.ok("%s (%s)", label, tildePath(path))
	} else {
		r.err("%s missing or not a directory: %s", label, tildePath(path))
	}
}

func checkRealDir(r *Report, label, path string) {
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

func checkSymlink(r *Report, label, linkPath, expectedTarget string) {
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

func checkProjects(r *Report, cfg *config.Config) {
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

		// Check repos.
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

		if len(m.Repos) == 0 {
			r.info("%s: no repos tracked", m.Name)
		}

		return nil
	})
}
