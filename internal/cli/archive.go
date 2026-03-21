package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:               "archive <query>",
	Short:             "Archive a project",
	Long:              "Move a project to the archive.\nGit repos that are clean and synced with their remote are deleted.\nAll other code is preserved in the archive.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runArchive,
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

	results, err := project.Archive(cfg, p)
	if err != nil {
		return err
	}

	for _, r := range results {
		if r.Deleted {
			ui.ok(fmt.Sprintf("Repo %s: deleted (clean, synced with remote)", r.Name))
		} else {
			ui.info(fmt.Sprintf("Repo %s: moved to archive (not safe to delete)", r.Name))
		}
	}

	ui.line()
	ui.ok("Project archived")
	return nil
}

var restoreCmd = &cobra.Command{
	Use:               "restore <query>",
	Short:             "Restore an archived project",
	Long:              "Move a project from the archive back to the active workspace.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeArchivedProjectQuery,
	RunE:              runRestore,
}

func runRestore(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	p, err := resolveOne(cfg, query, project.FindArchivedByQuery)
	if err != nil {
		return fmt.Errorf("no archived project found matching %q", query)
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading(fmt.Sprintf("Restoring %s/%s", p.Org, p.Name))

	if err := project.Restore(cfg, p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project restored")
	return nil
}

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:               "delete <query>",
	Short:             "Permanently delete a project",
	Long:              "Remove a project and its code directory entirely.\nRequires --force flag.",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runDelete,
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "confirm deletion")
}

func runDelete(cmd *cobra.Command, args []string) error {
	if !deleteForce {
		return fmt.Errorf("this will permanently delete the project and all its code — use --force to confirm")
	}

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
	ui.heading(fmt.Sprintf("Deleting %s/%s", p.Org, p.Name))

	if err := project.Delete(cfg, p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project deleted")
	return nil
}

// resolveOne finds exactly one project matching the query, or returns an error.
func resolveOne(cfg *config.Config, query string, fn func(*config.Config, string) ([]*project.Project, error)) (*project.Project, error) {
	matches, err := fn(cfg, query)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no project found matching %q", query)
	case 1:
		return matches[0], nil
	default:
		msg := fmt.Sprintf("multiple projects match %q:\n", query)
		for _, m := range matches {
			msg += fmt.Sprintf("  %s/%s\n", m.Org, m.Name)
		}
		return nil, fmt.Errorf("%s", msg)
	}
}
