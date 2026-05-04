package cli

import (
	"fmt"
	"path/filepath"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var newNoCode bool

var newCmd = &cobra.Command{
	Use:   "new <org/project> [url]",
	Short: "Create a new project",
	Long:  "Create a new project under the given organization.\nBy default, a code directory is created. Use --no-code to skip it.\nIf a URL is provided, the repo is cloned into the code directory.",
	Args:  cobra.RangeArgs(1, 2),
	ValidArgsFunction: newCompletionFunc,
	RunE:  runNew,
}

func init() {
	newCmd.Flags().BoolVar(&newNoCode, "no-code", false, "create project without a code directory")
}

func newCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		orgs := listOrgs()
		var completions []string
		for _, org := range orgs {
			completions = append(completions, org+"/")
		}
		return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	org, name, err := parseOrgProject(args[0])
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	var p *project.Project
	if newNoCode {
		p = project.ResolveNew(cfg, org, name)
	} else {
		p = project.ResolveNewWithCode(cfg, org, name)
	}

	ui.heading(fmt.Sprintf("Creating project: %s/%s", org, name))
	ui.info(fmt.Sprintf("Project: %s", tildePath(p.ProjectRoot)))

	if err := project.Create(p); err != nil {
		return err
	}

	// If URL provided, clone the repo.
	if len(args) == 2 {
		repoURL := args[1]
		repoName, err := project.CloneRepo(cfg, p, repoURL)
		if err != nil {
			return err
		}
		ui.ok(fmt.Sprintf("Cloned %s into %s/%s", repoName, org, name))
		ui.info(fmt.Sprintf("Code: %s", tildePath(filepath.Join(p.CodeRoot, repoName))))
	}

	ui.line()
	ui.ok("Project created")
	if newNoCode {
		ui.hint("Use 'hive add' to add repositories later")
	}
	return nil
}
