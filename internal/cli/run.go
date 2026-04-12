package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <query> <cmd...>",
	Short: "Run a command in a project directory",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runRun,
}

func init() {
	runCmd.Flags().Bool("each", false, "Run command in each repo under the project's code directory")
}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	each, _ := cmd.Flags().GetBool("each")

	p, err := resolveOne(cfg, args[0], project.FindByQuery)
	if err != nil {
		return err
	}

	command := args[1:]

	if !each {
		c := exec.Command(command[0], command[1:]...)
		c.Dir = p.ProjectRoot
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}

	if !p.HasCode() {
		return fmt.Errorf("project %s/%s has no code directory", p.Org, p.Name)
	}

	entries, err := os.ReadDir(p.CodeRoot)
	if err != nil {
		return fmt.Errorf("read code directory: %w", err)
	}

	var failures []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := p.CodeRoot + "/" + entry.Name()
		ui.heading(entry.Name())

		c := exec.Command(command[0], command[1:]...)
		c.Dir = dir
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			failures = append(failures, entry.Name())
			ui.fail(fmt.Sprintf("failed: %s", err))
		}
	}

	if len(failures) > 0 {
		ui.line()
		for _, name := range failures {
			ui.fail(name)
		}
		return fmt.Errorf("%d repo(s) failed", len(failures))
	}

	return nil
}
