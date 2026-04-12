package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/gitutil"
	"github.com/sakakibara/hive/internal/meta"
	"github.com/sakakibara/hive/internal/project"
)

// SyncChange describes a single metadata change detected during sync.
type SyncChange struct {
	Project  string
	RepoName string
	Action   string // "repo_added", "url_updated", "repo_removed"
	OldValue string
	NewValue string
}

// SymlinkFix describes a symlink repair performed during sync.
type SymlinkFix struct {
	Project string
	Action  string // "created", "fixed", "ok", "error"
	Error   string
}

// SyncResult holds the combined results of a sync operation.
type SyncResult struct {
	Changes     []SyncChange
	SymlinkFixes []SymlinkFix
}

// Sync scans all projects and performs two operations:
//  1. Fixes code symlinks for projects that have code.
//  2. Reconciles metadata with repos on disk (detects added, updated, and removed repos).
func Sync(cfg *config.Config) (*SyncResult, error) {
	projects, err := project.Scan(cfg)
	if err != nil {
		return nil, fmt.Errorf("scan projects: %w", err)
	}

	resolved := cfg.Resolved()
	result := &SyncResult{}

	for _, p := range projects {
		if !p.HasCode() {
			continue
		}

		// Step 1: Fix symlinks.
		fix := fixCodeSymlink(p)
		result.SymlinkFixes = append(result.SymlinkFixes, fix)

		// Step 2: Sync metadata with repos on disk.
		changes, err := syncProjectMetadata(p, resolved.Paths.Code)
		if err != nil {
			continue
		}
		result.Changes = append(result.Changes, changes...)
	}

	return result, nil
}

// fixCodeSymlink ensures the code symlink in a project root points to the correct target.
func fixCodeSymlink(p *project.Project) SymlinkFix {
	codeLinkPath := filepath.Join(p.ProjectRoot, "code")

	info, err := os.Lstat(codeLinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create missing symlink.
			if err := os.Symlink(p.CodeRoot, codeLinkPath); err != nil {
				return SymlinkFix{
					Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
					Action:  "error",
					Error:   fmt.Sprintf("create symlink: %v", err),
				}
			}
			return SymlinkFix{
				Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
				Action:  "created",
			}
		}
		return SymlinkFix{
			Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
			Action:  "error",
			Error:   err.Error(),
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		existing, _ := os.Readlink(codeLinkPath)
		if existing == p.CodeRoot {
			return SymlinkFix{
				Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
				Action:  "ok",
			}
		}
		// Remove stale symlink and recreate.
		if err := os.Remove(codeLinkPath); err != nil {
			return SymlinkFix{
				Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
				Action:  "error",
				Error:   fmt.Sprintf("remove stale symlink: %v", err),
			}
		}
		if err := os.Symlink(p.CodeRoot, codeLinkPath); err != nil {
			return SymlinkFix{
				Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
				Action:  "error",
				Error:   fmt.Sprintf("recreate symlink: %v", err),
			}
		}
		return SymlinkFix{
			Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
			Action:  "fixed",
		}
	}

	return SymlinkFix{
		Project: fmt.Sprintf("%s/%s", p.Org, p.Name),
		Action:  "error",
		Error:   fmt.Sprintf("%s exists but is not a symlink (%s); refusing to overwrite", codeLinkPath, info.Mode().Type()),
	}
}

// syncProjectMetadata detects repos added/updated/removed on disk vs metadata.
func syncProjectMetadata(p *project.Project, codeRoot string) ([]SyncChange, error) {
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	m, err := meta.Read(metaPath)
	if err != nil {
		return nil, err
	}

	codeDir := filepath.Join(codeRoot, m.CodeRel)
	if !fsutil.IsDir(codeDir) {
		return nil, nil
	}

	projectLabel := fmt.Sprintf("%s/%s", p.Org, p.Name)
	var changes []SyncChange
	dirty := false

	// Detect repos on disk.
	entries, err := os.ReadDir(codeDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		seen[name] = true

		repoPath := filepath.Join(codeDir, name)
		repoURL := gitutil.DetectRemoteURL(repoPath)

		existingURL, tracked := m.Repos[name]
		if !tracked {
			// New repo on disk, not in metadata.
			if m.Repos == nil {
				m.Repos = make(map[string]string)
			}
			m.Repos[name] = repoURL
			dirty = true
			changes = append(changes, SyncChange{
				Project:  projectLabel,
				RepoName: name,
				Action:   "repo_added",
				NewValue: repoURL,
			})
		} else if repoURL != "" && repoURL != existingURL {
			// URL changed.
			m.Repos[name] = repoURL
			dirty = true
			changes = append(changes, SyncChange{
				Project:  projectLabel,
				RepoName: name,
				Action:   "url_updated",
				OldValue: existingURL,
				NewValue: repoURL,
			})
		}
	}

	// Detect stale repos in metadata that no longer exist on disk.
	for name := range m.Repos {
		if !seen[name] {
			delete(m.Repos, name)
			dirty = true
			changes = append(changes, SyncChange{
				Project:  projectLabel,
				RepoName: name,
				Action:   "repo_removed",
			})
		}
	}

	if dirty {
		_ = meta.Write(metaPath, m)
	}

	return changes, nil
}
