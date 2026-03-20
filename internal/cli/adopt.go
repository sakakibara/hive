package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var adoptCmd = &cobra.Command{
	Use:   "adopt <org> <project> <path>",
	Short: "Adopt an existing repository into hive",
	Long:  "Move an existing local repository into hive's managed structure.",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: adoptCompletionFunc,
	RunE:  runAdopt,
}

func adoptCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 2 {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func runAdopt(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	org := args[0]
	name := args[1]
	sourcePath := args[2]

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	absPath, _ := filepath.Abs(sourcePath)
	ui.heading(fmt.Sprintf("Adopting %s as %s/%s", tildePath(absPath), org, name))

	p, err := project.Adopt(cfg, org, name, sourcePath)
	if err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Project root: %s", tildePath(p.ProjectRoot)))
	ui.ok(fmt.Sprintf("Code: %s", tildePath(p.CodeRoot)))
	for repoName, repoURL := range p.Repos {
		if repoURL != "" {
			ui.ok(fmt.Sprintf("Repo: %s (%s)", repoName, repoURL))
		} else {
			ui.ok(fmt.Sprintf("Repo: %s", repoName))
		}
	}
	ui.line()
	ui.ok("Project adopted")
	return nil
}
