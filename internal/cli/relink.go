package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var relinkCmd = &cobra.Command{
	Use:   "relink",
	Short: "Recreate code symlinks for all projects",
	RunE:  runRelink,
}

func runRelink(cmd *cobra.Command, args []string) error {
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

	ui.heading("Relinking projects")
	hasErrors := false

	for _, p := range projects {
		if !p.HasCode() {
			continue
		}
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := ensureOrFixCodeSymlink(codeLinkPath, p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}
		ui.ok(fmt.Sprintf("%s/%s → %s", p.Org, p.Name, tildePath(p.CodeRoot)))
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("relink completed with errors — check output above")
	}
	ui.ok("Relink complete")
	return nil
}

func ensureOrFixCodeSymlink(linkPath, target string) error {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Symlink(target, linkPath)
		}
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		existing, _ := os.Readlink(linkPath)
		if existing == target {
			return nil
		}
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove stale symlink: %w", err)
		}
		return os.Symlink(target, linkPath)
	}

	return fmt.Errorf("%s exists but is not a symlink (%s); refusing to overwrite", linkPath, info.Mode().Type())
}
