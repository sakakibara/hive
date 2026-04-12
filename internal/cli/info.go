package cli

import (
	"fmt"
	"strings"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:               "info <query>",
	Short:             "Show details about a project",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	p, err := resolveOne(cfg, args[0], project.FindByQuery)
	if err != nil {
		return err
	}

	ui.heading(p.Org + "/" + p.Name)
	ui.info("Path: " + tildePath(p.ProjectRoot))

	if p.HasCode() {
		ui.info("Code: " + tildePath(p.CodeRoot))
	}

	if p.Meta != nil && p.Meta.CreatedAt != "" {
		created := p.Meta.CreatedAt
		if idx := strings.Index(created, "T"); idx >= 0 {
			created = created[:idx]
		}
		ui.info("Created: " + created)
	}

	if len(p.Repos) > 0 {
		ui.info("Repos:")
		for name, url := range p.Repos {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s\n", name, url)
		}
	}

	return nil
}
