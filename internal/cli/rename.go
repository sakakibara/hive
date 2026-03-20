package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <query> <new-name>",
	Short: "Rename a project",
	Long:  "Rename a project, moving its directories and updating metadata.",
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
}

func runRename(cmd *cobra.Command, args []string) error {
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

	if err := project.Rename(cfg, p, newName); err != nil {
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

var renameOrgCmd = &cobra.Command{
	Use:   "rename-org <old-org> <new-org>",
	Short: "Rename an organization",
	Long:  "Rename an organization, moving all its projects and updating metadata.",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: renameOrgCompletionFunc,
	RunE:  runRenameOrg,
}

func renameOrgCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveDefault
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

	moved, err := project.RenameOrg(cfg, oldOrg, newOrg)
	if err != nil {
		return err
	}

	for _, p := range moved {
		ui.ok(fmt.Sprintf("%s/%s", p.Org, p.Name))
	}
	ui.line()
	ui.ok(fmt.Sprintf("Renamed %d project(s)", len(moved)))
	return nil
}
