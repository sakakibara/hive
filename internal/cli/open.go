package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:               "path <query>",
	Short:             "Print the project path",
	Long:              "Print the absolute path to the matching project.\nUsage: cd \"$(hive path myproject)\"",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runPath,
}

func runPath(cmd *cobra.Command, args []string) error {
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

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Interactively select a project with fzf",
	Long:  "List all projects and select one with fzf.\nPrints the selected project path.\nUsage with shell function: hi",
	Args:  cobra.NoArgs,
	RunE:  runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	projects, err := project.Scan(cfg)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		return fmt.Errorf("no projects found")
	}

	// Build lines for fzf: "org/name"
	var lines []string
	projectMap := make(map[string]*project.Project)
	for _, p := range projects {
		key := filepath.Join(p.Org, p.Name)
		lines = append(lines, key)
		projectMap[key] = p
	}

	input := strings.Join(lines, "\n")

	fzf := exec.Command("fzf", "--height=~50%", "--reverse")
	fzf.Stdin = strings.NewReader(input)
	fzf.Stderr = os.Stderr

	out, err := fzf.Output()
	if err != nil {
		// User cancelled fzf (exit code 130).
		return fmt.Errorf("no project selected")
	}

	selected := strings.TrimSpace(string(out))
	p, ok := projectMap[selected]
	if !ok {
		return fmt.Errorf("project not found: %s", selected)
	}

	fmt.Fprint(cmd.OutOrStdout(), p.ProjectRoot)
	return nil
}
