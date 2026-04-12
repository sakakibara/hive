package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/lifecycle"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var restoreAll bool

var restoreCmd = &cobra.Command{
	Use:               "restore [query]",
	Short:             "Restore an archived project",
	Long:              "Move a project from the archive back to the active workspace.\nWith --all, restore all projects (clone missing repos, recreate symlinks).",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeArchivedProjectQuery,
	RunE:              runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreAll, "all", false, "restore all projects (clone missing repos, recreate symlinks)")
}

func runRestore(cmd *cobra.Command, args []string) error {
	if restoreAll {
		return runRestoreAll(cmd)
	}

	if len(args) == 0 {
		return fmt.Errorf("query argument required (or use --all to restore all)")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	p, err := resolveOne(cfg, query, project.FindArchivedByQuery)
	if err != nil {
		return fmt.Errorf("no archived match found")
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading(fmt.Sprintf("Restoring %s/%s", p.Org, p.Name))

	if err := lifecycle.Restore(cfg, p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project restored")
	return nil
}

func runRestoreAll(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading("Restoring all projects")

	err = lifecycle.RestoreAll(cfg,
		func(org, name, repo string) {
			ui.info(fmt.Sprintf("Cloning %s/%s/%s", org, name, repo))
		},
		func(name string, err error) {
			ui.fail(fmt.Sprintf("%s: clone failed — %v", name, err))
		},
	)
	if err != nil {
		return err
	}

	ui.line()
	ui.ok("Restore complete")
	return nil
}
