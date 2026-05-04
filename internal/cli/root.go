package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hive",
	Short: "Manage your workspace layout",
	Long:  "hive is a CLI tool for managing projects, life documents, work documents, archives, and code across machines.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(pathCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(recentCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workspace not initialized — run 'hive workspace init' first")
		}
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// parseOrgProject splits "org/project" into org and project parts.
func parseOrgProject(arg string) (org, proj string, err error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected org/project format, got %q", arg)
	}
	return parts[0], parts[1], nil
}

// resolveOne finds exactly one project matching the query, or returns an error.
func resolveOne(cfg *config.Config, query string, fn func(*config.Config, string) ([]*project.Project, error)) (*project.Project, error) {
	matches, err := fn(cfg, query)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no match found")
	case 1:
		return matches[0], nil
	default:
		msg := "multiple matches:\n"
		for _, m := range matches {
			msg += fmt.Sprintf("  %s/%s\n", m.Org, m.Name)
		}
		return nil, fmt.Errorf("%s", msg)
	}
}

func completeProjectQuery(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	projects, err := project.Scan(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var completions []string
	for _, p := range projects {
		completions = append(completions, filepath.Join(p.Org, p.Name))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeArchivedProjectQuery(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	projects, err := project.ScanArchive(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var completions []string
	for _, p := range projects {
		completions = append(completions, filepath.Join(p.Org, p.Name))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func listOrgs() []string {
	cfg, err := loadConfig()
	if err != nil {
		return nil
	}
	resolved := cfg.Resolved()
	entries, err := os.ReadDir(resolved.Paths.Projects)
	if err != nil {
		return nil
	}
	var orgs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			orgs = append(orgs, e.Name())
		}
	}
	return orgs
}
