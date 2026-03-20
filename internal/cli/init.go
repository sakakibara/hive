package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize workspace directories and symlinks",
	Long:  "Detect storage mode, write config, and create the required directory layout.\nSafe to re-run: skips config creation if it already exists.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	var cfg *config.Config

	if config.Exists() {
		ui.info("Config already exists — using existing configuration")
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	} else {
		ui.heading("Detecting storage mode")
		var err error
		cfg, err = config.DetectAndCreate()
		if err != nil {
			return err
		}
		ui.ok(fmt.Sprintf("Storage mode: %s", cfg.Storage.Mode))
		ui.ok(fmt.Sprintf("Config written to %s", tildePath(config.DefaultConfigPath())))
	}

	resolved := cfg.Resolved()
	hasErrors := false

	if cfg.IsICloud() {
		hasErrors = initICloudMode(ui, cfg, resolved)
	} else {
		hasErrors = initLocalMode(ui, resolved)
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("workspace initialization incomplete — fix the issues above and re-run hive init")
	}
	ui.ok("Workspace initialized")
	return nil
}

func initICloudMode(ui *UI, cfg *config.Config, resolved *config.Config) bool {
	hasErrors := false

	ui.heading("Creating directories (icloud mode)")

	icloudDirs := []struct{ label, path string }{
		{"iCloud workspace root", resolved.Storage.ICloudRoot},
		{"iCloud projects", cfg.ICloudDir("projects")},
		{"iCloud life", cfg.ICloudDir("life")},
		{"iCloud work", cfg.ICloudDir("work")},
		{"iCloud archive", cfg.ICloudDir("archive")},
		{"Local code root", resolved.Paths.Code},
	}

	for _, d := range icloudDirs {
		if err := fsutil.EnsureDir(d.path); err != nil {
			ui.fail(fmt.Sprintf("%s — %v", d.label, err))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s (%s)", d.label, tildePath(d.path)))
		}
	}

	ui.heading("Creating symlinks")

	links := []struct{ label, link, target string }{
		{"~/Projects", resolved.Paths.Projects, cfg.ICloudDir("projects")},
		{"~/Life", resolved.Paths.Life, cfg.ICloudDir("life")},
		{"~/Work", resolved.Paths.Work, cfg.ICloudDir("work")},
		{"~/Archive", resolved.Paths.Archive, cfg.ICloudDir("archive")},
	}

	for _, l := range links {
		displayTarget := tildePath(l.target)
		displayLink := tildePath(l.link)
		if err := fsutil.EnsureSymlink(l.target, l.link); err != nil {
			ui.fail(fmt.Sprintf("%s → %s", displayLink, displayTarget))
			ui.hint(fmt.Sprintf("%s already exists as a %s", displayLink, descExisting(l.link)))
			ui.hint(fmt.Sprintf("Fix: move or rename %s, then re-run hive init", displayLink))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s → %s", displayLink, displayTarget))
		}
	}

	return hasErrors
}

func initLocalMode(ui *UI, resolved *config.Config) bool {
	hasErrors := false

	ui.heading("Creating directories (local mode)")

	dirs := []struct{ label, path string }{
		{"Projects", resolved.Paths.Projects},
		{"Life", resolved.Paths.Life},
		{"Work", resolved.Paths.Work},
		{"Archive", resolved.Paths.Archive},
		{"Code", resolved.Paths.Code},
	}

	for _, d := range dirs {
		if fsutil.IsSymlink(d.path) {
			ui.fail(fmt.Sprintf("%s (%s) is a symlink — local mode requires real directories", d.label, tildePath(d.path)))
			hasErrors = true
			continue
		}
		if err := fsutil.EnsureDir(d.path); err != nil {
			ui.fail(fmt.Sprintf("%s — %v", d.label, err))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s (%s)", d.label, tildePath(d.path)))
		}
	}

	return hasErrors
}
