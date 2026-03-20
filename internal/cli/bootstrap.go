package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Restore code directories and symlinks from project metadata",
	Long:  "Scan all .hive.json files under ~/Projects, clone missing repos, and recreate code symlinks.\nIntended for onboarding a new Mac.",
	RunE:  runBootstrap,
}

const maxConcurrentClones = 6

type cloneJob struct {
	project  *project.Project
	repoName string
	repoURL  string
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	if len(projects) == 0 {
		ui.info("No projects found")
		return nil
	}

	// Phase 1: collect missing repos to clone.
	var jobs []cloneJob
	for _, p := range projects {
		if !p.HasCode() {
			continue
		}
		for repoName, repoURL := range p.Repos {
			dest := filepath.Join(p.CodeRoot, repoName)
			if !fsutil.PathExists(dest) && repoURL != "" {
				jobs = append(jobs, cloneJob{project: p, repoName: repoName, repoURL: repoURL})
			}
		}
	}

	if len(jobs) > 0 {
		ui.heading(fmt.Sprintf("Cloning %d repos (max %d parallel)", len(jobs), maxConcurrentClones))
		cloneErrors := parallelCloneJobs(jobs, ui)
		for _, ce := range cloneErrors {
			ui.fail(fmt.Sprintf("%s: clone failed — %v", ce.name, ce.err))
		}
	}

	// Phase 2: ensure code dirs and symlinks.
	ui.heading("Setting up projects")
	hasErrors := false

	for _, p := range projects {
		if !p.HasCode() {
			ui.ok(fmt.Sprintf("%s/%s (no code)", p.Org, p.Name))
			continue
		}

		if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s: create code dir — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}

		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := ensureOrFixCodeSymlink(codeLinkPath, p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s: symlink — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}
		ui.ok(fmt.Sprintf("%s/%s → %s", p.Org, p.Name, tildePath(p.CodeRoot)))
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("bootstrap completed with errors — check output above")
	}
	ui.ok("Bootstrap complete")
	return nil
}

type cloneError struct {
	name string
	err  error
}

func parallelCloneJobs(jobs []cloneJob, ui *UI) []cloneError {
	sem := make(chan struct{}, maxConcurrentClones)
	var mu sync.Mutex
	var errors []cloneError

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j cloneJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ui.info(fmt.Sprintf("Cloning %s/%s/%s", j.project.Org, j.project.Name, j.repoName))
			if err := gitCloneForBootstrap(j.repoURL, filepath.Join(j.project.CodeRoot, j.repoName)); err != nil {
				mu.Lock()
				errors = append(errors, cloneError{name: j.repoName, err: err})
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()

	return errors
}

func gitCloneForBootstrap(url, dest string) error {
	if err := fsutil.EnsureDir(filepath.Dir(dest)); err != nil {
		return err
	}
	cmd := newGitCloneCmd(url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
