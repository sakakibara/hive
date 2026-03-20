package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <query>",
	Short: "Print the project path for shell use",
	Long:  "Print the path to the matching project.\nUsage: cd \"$(hive open myproject)\"",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	p, err := resolveOne(cfg, query, project.FindByQuery)
	if err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), p.ProjectRoot)
	return nil
}
