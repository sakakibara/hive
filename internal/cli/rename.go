package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/lifecycle"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var renameOrg bool

var renameCmd = &cobra.Command{
	Use:               "rename <query> <new-name>",
	Short:             "Rename a project or organization",
	Long:              "Rename a project, moving its directories and updating metadata.\nWith --org, rename an organization (args become <old-org> <new-org>).",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: renameCompletionFunc,
	RunE:              runRename,
}

func init() {
	renameCmd.Flags().BoolVar(&renameOrg, "org", false, "rename an organization instead of a project")
}

func renameCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	orgFlag, _ := cmd.Flags().GetBool("org")
	if orgFlag {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	return completeProjectQuery(cmd, args, toComplete)
}

func runRename(cmd *cobra.Command, args []string) error {
	if renameOrg {
		return runRenameOrg(cmd, args)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	newName := args[1]

	p, err := resolveOne(cfg, query, project.FindByQuery)
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading(fmt.Sprintf("Renaming %s/%s → %s/%s", p.Org, p.Name, p.Org, newName))

	if err := lifecycle.Rename(cfg, p, newName); err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Project: %s", tildePath(p.ProjectRoot)))
	if p.HasCode() {
		ui.ok(fmt.Sprintf("Code: %s", tildePath(p.CodeRoot)))
	}
	ui.line()
	ui.ok("Project renamed")
	return nil
}

func runRenameOrg(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	oldOrg := args[0]
	newOrg := args[1]

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	ui.heading(fmt.Sprintf("Renaming org %s → %s", oldOrg, newOrg))

	if err := lifecycle.RenameOrg(cfg, oldOrg, newOrg); err != nil {
		return err
	}

	ui.line()
	ui.ok("Organization renamed")
	return nil
}
