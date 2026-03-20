package cli

import (
	"fmt"
	"strings"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	if len(projects) == 0 {
		ui.info("No projects found")
		return nil
	}

	w := cmd.OutOrStdout()

	maxName, maxOrg := 4, 3
	for _, p := range projects {
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
		if len(p.Org) > maxOrg {
			maxOrg = len(p.Org)
		}
	}

	format := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", maxName, maxOrg)
	fmt.Fprintf(w, format, "NAME", "ORG", "CODE")
	fmt.Fprintf(w, format,
		strings.Repeat("-", maxName),
		strings.Repeat("-", maxOrg),
		strings.Repeat("-", 4))

	for _, p := range projects {
		code := "-"
		if p.HasCode() {
			repos := len(p.Repos)
			if repos > 0 {
				code = fmt.Sprintf("%d repo(s)", repos)
			} else {
				code = "yes"
			}
		}
		fmt.Fprintf(w, format, p.Name, p.Org, code)
	}

	return nil
}
