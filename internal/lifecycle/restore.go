package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/project"
)

// Restore moves an archived project back to the active projects directory
// and recreates the code symlink.
func Restore(cfg *config.Config, p *project.Project) error {
	resolved := cfg.Resolved()

	// Compute where the project should live in the active tree.
	activeDest := filepath.Join(resolved.Paths.Projects, p.Org, p.Name)

	if msg := fsutil.PathConflict(activeDest); msg != "" {
		return fmt.Errorf("restore destination: %s", msg)
	}

	// If archived code exists, move it back.
	if p.HasCode() {
		archiveCodeDir := filepath.Join(resolved.Paths.Archive, "code", p.CodeRel)
		if fsutil.IsDir(archiveCodeDir) {
			if err := os.MkdirAll(filepath.Dir(p.CodeRoot), 0755); err != nil {
				return fmt.Errorf("create code parent: %w", err)
			}
			if err := fsutil.MoveDir(archiveCodeDir, p.CodeRoot); err != nil {
				return fmt.Errorf("restore code directory: %w", err)
			}
			fsutil.CleanEmptyParents(archiveCodeDir, filepath.Join(resolved.Paths.Archive, "code"))
		}
	}

	// Move project root back to active.
	if err := os.MkdirAll(filepath.Dir(activeDest), 0755); err != nil {
		return fmt.Errorf("create project parent: %w", err)
	}
	if err := fsutil.MoveDir(p.ProjectRoot, activeDest); err != nil {
		return fmt.Errorf("restore project: %w", err)
	}

	// Clean up empty archive parents.
	archiveProjectsDir := filepath.Join(resolved.Paths.Archive, "projects")
	fsutil.CleanEmptyParents(p.ProjectRoot, archiveProjectsDir)

	// Recreate code symlink.
	if p.HasCode() {
		codeLinkPath := filepath.Join(activeDest, "code")
		if err := ensureOrFixSymlink(codeLinkPath, p.CodeRoot); err != nil {
			return fmt.Errorf("recreate code symlink: %w", err)
		}
	}

	return nil
}

// RestoreAll scans all .hive.json files in the projects tree, clones missing
// repos in parallel (max 6 concurrent), and recreates code dirs and symlinks.
// This replaces the old bootstrap command.
func RestoreAll(
	cfg *config.Config,
	onCloneStart func(org, name, repo string),
	onCloneError func(name string, err error),
) error {
	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	resolved := cfg.Resolved()

	type cloneJob struct {
		org      string
		name     string
		repoName string
		url      string
		dest     string
	}

	var jobs []cloneJob

	for _, p := range projects {
		if !p.HasCode() {
			continue
		}

		// Ensure code directory exists.
		if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
			continue
		}

		// Recreate code symlink.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		_ = ensureOrFixSymlink(codeLinkPath, p.CodeRoot)

		// Queue clone jobs for missing repos.
		for repoName, repoURL := range p.Repos {
			dest := filepath.Join(p.CodeRoot, repoName)
			if fsutil.PathExists(dest) {
				continue
			}
			jobs = append(jobs, cloneJob{
				org:      p.Org,
				name:     p.Name,
				repoName: repoName,
				url:      repoURL,
				dest:     dest,
			})
		}

		// Also ensure code directory is under the code root.
		_ = fsutil.EnsureDir(filepath.Join(resolved.Paths.Code, p.CodeRel))
	}

	// Clone missing repos in parallel with a semaphore of 6.
	const maxConcurrent = 6
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j cloneJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if onCloneStart != nil {
				onCloneStart(j.org, j.name, j.repoName)
			}

			cmd := newGitCloneCmd(j.url, j.dest)
			if err := cmd.Run(); err != nil {
				if onCloneError != nil {
					onCloneError(j.repoName, err)
				}
			}
		}(job)
	}

	wg.Wait()
	return nil
}

// ensureOrFixSymlink creates a symlink at linkPath pointing to target,
// fixing it if it exists but points to the wrong target.
func ensureOrFixSymlink(linkPath, target string) error {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Symlink(target, linkPath)
		}
		return err
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists but is not a symlink", linkPath)
	}

	existing, err := os.Readlink(linkPath)
	if err != nil {
		return err
	}

	if existing == target {
		return nil
	}

	// Symlink points to wrong target — fix it.
	if err := os.Remove(linkPath); err != nil {
		return err
	}
	return os.Symlink(target, linkPath)
}
