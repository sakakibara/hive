package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sakakibara/hive/internal/config"
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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(relinkCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(renameOrgCmd)
	rootCmd.AddCommand(syncCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workspace not initialized — run 'hive init' first")
		}
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
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
