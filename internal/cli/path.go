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

var pathInteractive bool

var pathCmd = &cobra.Command{
	Use:               "path [query]",
	Short:             "Print the project path",
	Long:              "Print the absolute path to the matching project.\nWith -i, launch fzf to interactively select a project.\nUsage: cd \"$(hive path myproject)\"",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeProjectQuery,
	RunE:              runPath,
}

func init() {
	pathCmd.Flags().BoolVarP(&pathInteractive, "interactive", "i", false, "select project interactively with fzf")
}

func runPath(cmd *cobra.Command, args []string) error {
	if pathInteractive {
		return runPathInteractive(cmd)
	}

	if len(args) == 0 {
		return fmt.Errorf("query argument required (or use -i for interactive selection)")
	}

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

func runPathInteractive(cmd *cobra.Command) error {
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
