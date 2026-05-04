package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/lifecycle"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

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

	if err := lifecycle.Delete(cfg, p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project deleted")
	return nil
}
