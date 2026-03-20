package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <org> <project>",
	Short: "Create a new project",
	Long:  "Create a new project under the given organization.\nThe project starts without code. Use 'hive clone' to add repositories.",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: newCompletionFunc,
	RunE:  runNew,
}

func newCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	org := args[0]
	name := args[1]

	p := project.ResolveNew(cfg, org, name)
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading(fmt.Sprintf("Creating project: %s/%s", org, name))
	ui.info(fmt.Sprintf("Project: %s", tildePath(p.ProjectRoot)))

	if err := project.Create(p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project created")
	ui.hint("Use 'hive clone' to add repositories")
	return nil
}
