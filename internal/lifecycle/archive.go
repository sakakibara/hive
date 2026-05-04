package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/gitutil"
	"github.com/sakakibara/hive/internal/project"
)

// RepoArchiveResult records the outcome for a single repo during archive.
type RepoArchiveResult struct {
	Name    string
	Deleted bool
}

// ArchiveResult records the outcome of archiving a project.
type ArchiveResult struct {
	Repos []RepoArchiveResult
}

// FindSafeToDeleteRepos scans the code directory entries and returns names
// where gitutil.SafeToDelete() is true (clean, pushed, has remote).
func FindSafeToDeleteRepos(p *project.Project) []string {
	if p.CodeRoot == "" {
		return nil
	}
	entries, err := os.ReadDir(p.CodeRoot)
	if err != nil {
		return nil
	}
	var safe []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(p.CodeRoot, entry.Name())
		if gitutil.SafeToDelete(repoPath) {
			safe = append(safe, entry.Name())
		}
	}
	return safe
}

// Archive moves a project from the active projects directory to the archive.
// By default all code is preserved (moved to archive). If prune is true,
// repos whose names appear in pruneNames are deleted instead of archived.
func Archive(cfg *config.Config, p *project.Project, prune bool, pruneNames ...string) (*ArchiveResult, error) {
	resolved := cfg.Resolved()
	archiveProjectsDir := filepath.Join(resolved.Paths.Archive, "projects")

	// Compute archive destination for the project root.
	rel, err := filepath.Rel(resolved.Paths.Projects, p.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("compute relative path: %w", err)
	}
	archiveDest := filepath.Join(archiveProjectsDir, rel)

	// Check for conflicts at the archive destination.
	if msg := fsutil.PathConflict(archiveDest); msg != "" {
		return nil, fmt.Errorf("archive destination: %s", msg)
	}

	// Build prune set.
	pruneSet := make(map[string]bool)
	if prune {
		for _, name := range pruneNames {
			pruneSet[name] = true
		}
	}

	// Remove code symlink from project root (it will be stale after move).
	codeLinkPath := filepath.Join(p.ProjectRoot, "code")
	if fsutil.IsSymlink(codeLinkPath) {
		os.Remove(codeLinkPath)
	}

	// Process code directory: prune or move each repo.
	var result ArchiveResult
	if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
		archiveCodeDest := filepath.Join(resolved.Paths.Archive, "code", p.CodeRel)
		result.Repos = archiveCodeDir(p.CodeRoot, archiveCodeDest, pruneSet)
	}

	// Move project root to archive.
	if err := os.MkdirAll(filepath.Dir(archiveDest), 0755); err != nil {
		return nil, fmt.Errorf("create archive directory: %w", err)
	}
	if err := fsutil.MoveDir(p.ProjectRoot, archiveDest); err != nil {
		return nil, fmt.Errorf("move project to archive: %w", err)
	}

	// Clean up empty parent directories.
	fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)
	if p.HasCode() {
		fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)
	}

	return &result, nil
}

// archiveCodeDir processes the code directory entries, pruning or moving each.
func archiveCodeDir(codeRoot, archiveDest string, pruneSet map[string]bool) []RepoArchiveResult {
	var results []RepoArchiveResult
	entries, err := os.ReadDir(codeRoot)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		repoPath := filepath.Join(codeRoot, name)
		if pruneSet[name] {
			os.RemoveAll(repoPath)
			results = append(results, RepoArchiveResult{Name: name, Deleted: true})
		} else {
			dest := filepath.Join(archiveDest, name)
			os.MkdirAll(filepath.Dir(dest), 0755)
			if err := fsutil.MoveDir(repoPath, dest); err == nil {
				results = append(results, RepoArchiveResult{Name: name, Deleted: false})
			}
		}
	}
	if fsutil.IsEmptyDir(codeRoot) {
		os.Remove(codeRoot)
	}
	return results
}
