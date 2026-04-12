package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/lifecycle"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var archivePrune bool

var archiveCmd = &cobra.Command{
	Use:               "archive <query>",
	Short:             "Archive a project",
	Long:              "Move a project to the archive.\nWith --prune, repos that are clean and synced with their remote are deleted instead of archived.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runArchive,
}

func init() {
	archiveCmd.Flags().BoolVar(&archivePrune, "prune", false, "delete repos that are safe to delete (clean, pushed)")
}

func runArchive(cmd *cobra.Command, args []string) error {
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
	ui.heading(fmt.Sprintf("Archiving %s/%s", p.Org, p.Name))

	var pruneNames []string
	if archivePrune {
		safe := lifecycle.FindSafeToDeleteRepos(p)
		if len(safe) > 0 {
			ui.info("The following repos are clean and synced — they will be deleted:")
			for _, name := range safe {
				ui.info(fmt.Sprintf("  %s", name))
			}
			if !ui.confirm("Delete these repos?") {
				ui.info("Archiving without pruning")
			} else {
				pruneNames = safe
			}
		}
	}

	result, err := lifecycle.Archive(cfg, p, len(pruneNames) > 0, pruneNames...)
	if err != nil {
		return err
	}

	for _, r := range result.Repos {
		if r.Deleted {
			ui.ok(fmt.Sprintf("Repo %s: deleted (clean, synced with remote)", r.Name))
		} else {
			ui.info(fmt.Sprintf("Repo %s: moved to archive", r.Name))
		}
	}

	ui.line()
	ui.ok("Project archived")
	return nil
}
