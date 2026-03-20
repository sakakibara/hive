package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/config"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Show storage configuration",
	RunE:  runStorageShow,
}

func runStorageShow(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	resolved := cfg.Resolved()
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading("Storage configuration")
	ui.info(fmt.Sprintf("Mode:     %s", cfg.Storage.Mode))
	if cfg.IsICloud() {
		ui.info(fmt.Sprintf("iCloud:   %s", tildePath(resolved.Storage.ICloudRoot)))
	}
	ui.info(fmt.Sprintf("Projects: %s", tildePath(resolved.Paths.Projects)))
	ui.info(fmt.Sprintf("Life:     %s", tildePath(resolved.Paths.Life)))
	ui.info(fmt.Sprintf("Work:     %s", tildePath(resolved.Paths.Work)))
	ui.info(fmt.Sprintf("Archive:  %s", tildePath(resolved.Paths.Archive)))
	ui.info(fmt.Sprintf("Code:     %s", tildePath(resolved.Paths.Code)))
	ui.info(fmt.Sprintf("Config:   %s", tildePath(config.DefaultConfigPath())))

	return nil
}
