package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:               "edit <query>",
	Short:             "Open a project in your editor",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runEdit,
}

func runEdit(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		return fmt.Errorf("$EDITOR is not set")
	}

	p, err := resolveOne(cfg, args[0], project.FindByQuery)
	if err != nil {
		return err
	}

	c := exec.Command(editor, p.ProjectRoot)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
