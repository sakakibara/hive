package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/gitutil"
	"github.com/sakakibara/hive/internal/meta"
)

// SyncChange describes a single change made by Sync.
type SyncChange struct {
	Project  string
	RepoName string
	Action   string // "url_updated", "repo_added", "repo_removed"
	OldValue string
	NewValue string
}

// Sync scans all projects and reconciles metadata with the actual state on disk.
// It detects remote URLs from git repos and registers untracked repos.
func Sync(cfg *config.Config) ([]SyncChange, error) {
	projects, err := Scan(cfg)
	if err != nil {
		return nil, err
	}

	var changes []SyncChange

	for _, p := range projects {
		if !p.HasCode() || !fsutil.IsDir(p.CodeRoot) {
			continue
		}

		metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
		m, err := meta.Read(metaPath)
		if err != nil {
			continue
		}

		dirty := false

		// Check repos on disk.
		entries, err := os.ReadDir(p.CodeRoot)
		if err != nil {
			continue
		}

		seen := make(map[string]bool)
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			name := entry.Name()
			seen[name] = true
			repoPath := filepath.Join(p.CodeRoot, name)
			actualURL := gitutil.DetectRemoteURL(repoPath)

			trackedURL, tracked := m.Repos[name]
			if !tracked {
				m.AddRepo(name, actualURL)
				dirty = true
				changes = append(changes, SyncChange{
					Project:  p.Org + "/" + p.Name,
					RepoName: name,
					Action:   "repo_added",
					NewValue: actualURL,
				})
			} else if actualURL != "" && actualURL != trackedURL {
				m.Repos[name] = actualURL
				dirty = true
				changes = append(changes, SyncChange{
					Project:  p.Org + "/" + p.Name,
					RepoName: name,
					Action:   "url_updated",
					OldValue: trackedURL,
					NewValue: actualURL,
				})
			}
		}

		// Remove tracked repos that no longer exist on disk.
		for name := range m.Repos {
			if !seen[name] {
				delete(m.Repos, name)
				dirty = true
				changes = append(changes, SyncChange{
					Project:  p.Org + "/" + p.Name,
					RepoName: name,
					Action:   "repo_removed",
				})
			}
		}

		if dirty {
			meta.Write(metaPath, m)
		}
	}

	return changes, nil
}
