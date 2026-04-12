package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/lifecycle"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:               "backup <query>",
	Short:             "Back up a project to a compressed archive",
	Long:              "Create a .tar.gz archive of a project's code directory (iCloud mode)\nor both the project and code directories (local mode).",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runBackup,
}

func runBackup(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	p, err := resolveOne(cfg, query, project.FindByQuery)
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading(fmt.Sprintf("Backing up: %s/%s", p.Org, p.Name))

	outPath, err := lifecycle.CreateBackup(cfg, p, cfg.BackupDir())
	if err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Backup created: %s", tildePath(outPath)))
	return nil
}
