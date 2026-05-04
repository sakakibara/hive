package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/workspace"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Workspace maintenance commands",
	Long:  "Commands for initializing, diagnosing, and maintaining the hive workspace.",
}

func init() {
	workspaceCmd.AddCommand(wsInitCmd)
	workspaceCmd.AddCommand(wsDoctorCmd)
	workspaceCmd.AddCommand(wsSyncCmd)
	workspaceCmd.AddCommand(wsStorageCmd)
}

var wsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize workspace directories and symlinks",
	Long:  "Detect storage mode, write config, and create the required directory layout.\nSafe to re-run: skips config creation if it already exists.",
	RunE:  runWsInit,
}

func runWsInit(cmd *cobra.Command, args []string) error {
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

	hasErrors := false

	if cfg.IsICloud() {
		hasErrors = initICloudMode(ui, cfg)
	} else {
		hasErrors = initLocalMode(ui, cfg)
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("workspace initialization incomplete — fix the issues above and re-run hive workspace init")
	}
	ui.ok("Workspace initialized")
	return nil
}

func initICloudMode(ui *UI, cfg *config.Config) bool {
	hasErrors := false

	ui.heading("Creating directories (icloud mode)")
	for _, d := range workspace.ICloudDirs(cfg) {
		if err := workspace.EnsureDir(d.Path); err != nil {
			ui.fail(fmt.Sprintf("%s — %v", d.Label, err))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s (%s)", d.Label, tildePath(d.Path)))
		}
	}

	ui.heading("Creating symlinks")
	for _, l := range workspace.ICloudLinks(cfg) {
		displayTarget := tildePath(l.Target)
		displayLink := tildePath(l.Link)
		if err := workspace.EnsureSymlink(l.Target, l.Link); err != nil {
			ui.fail(fmt.Sprintf("%s → %s", displayLink, displayTarget))
			ui.hint(fmt.Sprintf("%s already exists as a %s", displayLink, descExisting(l.Link)))
			ui.hint(fmt.Sprintf("Fix: move or rename %s, then re-run hive workspace init", displayLink))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s → %s", displayLink, displayTarget))
		}
	}

	return hasErrors
}

func initLocalMode(ui *UI, cfg *config.Config) bool {
	hasErrors := false

	ui.heading("Creating directories (local mode)")
	for _, d := range workspace.LocalDirs(cfg) {
		if workspace.IsSymlink(d.Path) {
			ui.fail(fmt.Sprintf("%s (%s) is a symlink — local mode requires real directories", d.Label, tildePath(d.Path)))
			hasErrors = true
			continue
		}
		if err := workspace.EnsureDir(d.Path); err != nil {
			ui.fail(fmt.Sprintf("%s — %v", d.Label, err))
			hasErrors = true
		} else {
			ui.ok(fmt.Sprintf("%s (%s)", d.Label, tildePath(d.Path)))
		}
	}

	return hasErrors
}

var wsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose workspace health",
	RunE:  runWsDoctor,
}

func runWsDoctor(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading("Checking workspace health")
	ui.info(fmt.Sprintf("Storage mode: %s", cfg.Storage.Mode))
	report := workspace.Doctor(cfg)

	for _, r := range report.Results {
		switch r.Level {
		case workspace.DiagOK:
			ui.ok(r.Message)
		case workspace.DiagInfo:
			ui.info(r.Message)
		case workspace.DiagWarn:
			ui.warn(r.Message)
		case workspace.DiagErr:
			ui.fail(r.Message)
		}
	}

	ui.line()
	if report.HasErrors() {
		return fmt.Errorf("workspace has errors — see above for details")
	}
	ui.ok("Workspace is healthy")
	return nil
}

var wsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync project metadata and fix symlinks",
	Long:  "Detect repo URLs from git remotes, register untracked repos, remove stale entries, and fix code symlinks.",
	RunE:  runWsSync,
}

func runWsSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	result, err := workspace.Sync(cfg)
	if err != nil {
		return err
	}

	// Render symlink fixes.
	hasSymlinkChanges := false
	for _, fix := range result.SymlinkFixes {
		switch fix.Action {
		case "created":
			ui.ok(fmt.Sprintf("%s: symlink created", fix.Project))
			hasSymlinkChanges = true
		case "fixed":
			ui.ok(fmt.Sprintf("%s: symlink fixed", fix.Project))
			hasSymlinkChanges = true
		case "error":
			ui.fail(fmt.Sprintf("%s: symlink error — %s", fix.Project, fix.Error))
			hasSymlinkChanges = true
		}
	}

	// Render metadata changes.
	if len(result.Changes) == 0 && !hasSymlinkChanges {
		ui.ok("Everything is up to date")
		return nil
	}

	for _, c := range result.Changes {
		switch c.Action {
		case "repo_added":
			if c.NewValue != "" {
				ui.ok(fmt.Sprintf("%s: added repo %s (%s)", c.Project, c.RepoName, c.NewValue))
			} else {
				ui.ok(fmt.Sprintf("%s: added repo %s", c.Project, c.RepoName))
			}
		case "url_updated":
			ui.ok(fmt.Sprintf("%s: updated %s URL → %s", c.Project, c.RepoName, c.NewValue))
		case "repo_removed":
			ui.info(fmt.Sprintf("%s: removed stale repo %s", c.Project, c.RepoName))
		}
	}

	total := len(result.Changes)
	if total > 0 {
		ui.line()
		ui.ok(fmt.Sprintf("Synced %d change(s)", total))
	}
	return nil
}

var wsStorageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Show storage configuration",
	RunE:  runWsStorage,
}

func runWsStorage(cmd *cobra.Command, args []string) error {
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

