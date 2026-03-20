package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync project metadata with actual state on disk",
	Long:  "Detect repo URLs from git remotes, register untracked repos, and remove stale entries.",
	RunE:  runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading("Syncing metadata")

	changes, err := project.Sync(cfg)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		ui.ok("Everything is up to date")
		return nil
	}

	for _, c := range changes {
		switch c.Action {
		case "repo_added":
			if c.NewValue != "" {
				ui.ok(fmt.Sprintf("%s: added repo %s (%s)", c.Project, c.RepoName, c.NewValue))
			} else {
				ui.ok(fmt.Sprintf("%s: added repo %s", c.Project, c.RepoName))
			}
		case "url_updated":
			ui.ok(fmt.Sprintf("%s: updated %s URL → %s", c.Project, c.RepoName, c.NewValue))
		case "repo_removed":
			ui.info(fmt.Sprintf("%s: removed stale repo %s", c.Project, c.RepoName))
		}
	}

	ui.line()
	ui.ok(fmt.Sprintf("Synced %d change(s)", len(changes)))
	return nil
}
