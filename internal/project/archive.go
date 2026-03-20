package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/gitutil"
)

// RepoArchiveResult describes what happened to a repo during archiving.
type RepoArchiveResult struct {
	Name    string
	Deleted bool // true if safely deleted, false if moved to archive
}

// Archive moves a project from Projects to Archive/projects.
// Git repos that are clean and synced with their remote are deleted.
// All other code (dirty repos, no remote, non-git) is moved into the archive.
func Archive(cfg *config.Config, p *Project) ([]RepoArchiveResult, error) {
	resolved := cfg.Resolved()
	archiveDest := filepath.Join(resolved.Paths.Archive, "projects", p.Org, p.Name)

	if msg := fsutil.PathConflict(archiveDest); msg != "" {
		return nil, fmt.Errorf("archive destination: %s", msg)
	}

	var results []RepoArchiveResult

	// Handle code directory before moving the project.
	if p.HasCode() {
		// Remove the code symlink from the project root.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if fsutil.IsSymlink(codeLinkPath) {
			if err := os.Remove(codeLinkPath); err != nil {
				return nil, fmt.Errorf("remove code symlink: %w", err)
			}
		}

		if fsutil.IsDir(p.CodeRoot) {
			archiveCodeDest := filepath.Join(archiveDest, "code")
			results = archiveCodeDir(p.CodeRoot, archiveCodeDest)
		}
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

	return results, nil
}

// archiveCodeDir processes each entry in the code directory.
// Safe-to-delete repos are removed; everything else is moved to archiveDest.
func archiveCodeDir(codeRoot, archiveDest string) []RepoArchiveResult {
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

		if gitutil.SafeToDelete(repoPath) {
			os.RemoveAll(repoPath)
			results = append(results, RepoArchiveResult{Name: name, Deleted: true})
		} else {
			// Move to archive.
			dest := filepath.Join(archiveDest, name)
			os.MkdirAll(filepath.Dir(dest), 0755)
			if err := fsutil.MoveDir(repoPath, dest); err == nil {
				results = append(results, RepoArchiveResult{Name: name, Deleted: false})
			}
		}
	}

	// Remove the code root if it's now empty.
	if fsutil.IsEmptyDir(codeRoot) {
		os.Remove(codeRoot)
	}

	return results
}

// Restore moves a project from Archive/projects back to Projects and Code.
func Restore(cfg *config.Config, p *Project) error {
	resolved := cfg.Resolved()
	archiveRoot := filepath.Join(resolved.Paths.Archive, "projects", p.Org, p.Name)
	projectDest := filepath.Join(resolved.Paths.Projects, p.Org, p.Name)

	if msg := fsutil.PathConflict(projectDest); msg != "" {
		return fmt.Errorf("project destination: %s", msg)
	}

	// Check if archived project has a code directory (real dir, not symlink).
	archivedCodeDir := filepath.Join(archiveRoot, "code")
	hasArchivedCode := fsutil.IsDir(archivedCodeDir) && !fsutil.IsSymlink(archivedCodeDir)

	if hasArchivedCode && p.HasCode() {
		codeTarget := filepath.Join(resolved.Paths.Code, p.CodeRel)
		if msg := fsutil.PathConflict(codeTarget); msg != "" {
			return fmt.Errorf("code destination: %s", msg)
		}

		// Move code out of archive to Code root.
		if err := os.MkdirAll(filepath.Dir(codeTarget), 0755); err != nil {
			return fmt.Errorf("create code directory: %w", err)
		}
		if err := fsutil.MoveDir(archivedCodeDir, codeTarget); err != nil {
			return fmt.Errorf("restore code directory: %w", err)
		}
	}

	// Move project root back.
	if err := os.MkdirAll(filepath.Dir(projectDest), 0755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}
	if err := fsutil.MoveDir(archiveRoot, projectDest); err != nil {
		return fmt.Errorf("restore project: %w", err)
	}

	// Recreate code symlink if project has code.
	if p.HasCode() {
		codeTarget := filepath.Join(resolved.Paths.Code, p.CodeRel)
		codeLinkPath := filepath.Join(projectDest, "code")
		if err := fsutil.EnsureSymlink(codeTarget, codeLinkPath); err != nil {
			return fmt.Errorf("recreate code symlink: %w", err)
		}
	}

	// Clean up empty archive parent directories.
	fsutil.CleanEmptyParents(archiveRoot, filepath.Join(resolved.Paths.Archive, "projects"))

	return nil
}

// Delete removes a project and its code directory entirely.
func Delete(cfg *config.Config, p *Project) error {
	resolved := cfg.Resolved()

	if err := os.RemoveAll(p.ProjectRoot); err != nil {
		return fmt.Errorf("remove project root: %w", err)
	}

	if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
		if err := os.RemoveAll(p.CodeRoot); err != nil {
			return fmt.Errorf("remove code root: %w", err)
		}
	}

	fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)
	if p.HasCode() {
		fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)
	}

	return nil
}

// ScanArchive finds all archived projects under Archive/projects.
func ScanArchive(cfg *config.Config) ([]*Project, error) {
	resolved := cfg.Resolved()
	archiveProjectsDir := filepath.Join(resolved.Paths.Archive, "projects")
	return scanDir(archiveProjectsDir, resolved.Paths.Code)
}

// FindArchivedByQuery returns archived projects matching the given query.
func FindArchivedByQuery(cfg *config.Config, query string) ([]*Project, error) {
	all, err := ScanArchive(cfg)
	if err != nil {
		return nil, err
	}
	return filterByQuery(all, query), nil
}
