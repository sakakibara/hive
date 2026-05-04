package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:               "add <query> <url>",
	Short:             "Clone a repository into a project",
	Long:              "Clone a git repository into an existing project's code directory.\nThe repo name is derived from the URL.",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runAdd,
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	repoURL := args[1]

	p, err := resolveOne(cfg, query, project.FindByQuery)
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	repoName, err := project.CloneRepo(cfg, p, repoURL)
	if err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Cloned %s into %s/%s", repoName, p.Org, p.Name))
	ui.info(fmt.Sprintf("Code: %s", tildePath(filepath.Join(p.CodeRoot, repoName))))
	return nil
}
