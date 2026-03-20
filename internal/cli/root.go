package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/doctor"
	"github.com/sakakibara/hive/internal/fsutil"
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

// --- nest init ---

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

// --- nest new ---

var newCmd = &cobra.Command{
	Use:   "new <org> <project>",
	Short: "Create a new project",
	Long:  "Create a new project under the given organization.\nThe project starts without code. Use 'hive clone' to add repositories.",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: newCompletionFunc,
	RunE:  runNew,
}

func newCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	org := args[0]
	name := args[1]

	p := project.ResolveNew(cfg, org, name)
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading(fmt.Sprintf("Creating project: %s/%s", org, name))
	ui.info(fmt.Sprintf("Project: %s", tildePath(p.ProjectRoot)))

	if err := project.Create(p); err != nil {
		return err
	}

	ui.line()
	ui.ok("Project created")
	ui.hint("Use 'hive clone' to add repositories")
	return nil
}

// --- nest clone ---

var cloneCmd = &cobra.Command{
	Use:   "clone <query> <repo_url>",
	Short: "Clone a repository into a project",
	Long:  "Clone a git repository into an existing project's code directory.\nThe repo name is derived from the URL.",
	Args:  cobra.ExactArgs(2),
	RunE:  runClone,
}

func runClone(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	query := args[0]
	repoURL := args[1]

	matches, err := project.FindByQuery(cfg, query)
	if err != nil {
		return err
	}

	switch len(matches) {
	case 0:
		return fmt.Errorf("no project found matching %q", query)
	case 1:
		// ok
	default:
		msg := fmt.Sprintf("multiple projects match %q:\n", query)
		for _, m := range matches {
			msg += fmt.Sprintf("  %s/%s\n", m.Org, m.Name)
		}
		return fmt.Errorf("%s", msg)
	}

	p := matches[0]
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	repoName, err := project.CloneRepo(cfg, p, repoURL)
	if err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Cloned %s into %s/%s", repoName, p.Org, p.Name))
	ui.info(fmt.Sprintf("Code: %s", tildePath(filepath.Join(p.CodeRoot, repoName))))
	return nil
}

// --- nest adopt ---

var adoptCmd = &cobra.Command{
	Use:   "adopt <org> <project> <path>",
	Short: "Adopt an existing repository into hive",
	Long:  "Move an existing local repository into hive's managed structure.",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: adoptCompletionFunc,
	RunE:  runAdopt,
}

func adoptCompletionFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return listOrgs(), cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 2 {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}
	return nil, cobra.ShellCompDirectiveDefault
}

func runAdopt(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	org := args[0]
	name := args[1]
	sourcePath := args[2]

	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())
	absPath, _ := filepath.Abs(sourcePath)
	ui.heading(fmt.Sprintf("Adopting %s as %s/%s", tildePath(absPath), org, name))

	p, err := project.Adopt(cfg, org, name, sourcePath)
	if err != nil {
		return err
	}

	ui.ok(fmt.Sprintf("Project root: %s", tildePath(p.ProjectRoot)))
	ui.ok(fmt.Sprintf("Code: %s", tildePath(p.CodeRoot)))
	for repoName, repoURL := range p.Repos {
		if repoURL != "" {
			ui.ok(fmt.Sprintf("Repo: %s (%s)", repoName, repoURL))
		} else {
			ui.ok(fmt.Sprintf("Repo: %s", repoName))
		}
	}
	ui.line()
	ui.ok("Project adopted")
	return nil
}

// --- nest bootstrap ---

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Restore code directories and symlinks from project metadata",
	Long:  "Scan all .hive.json files under ~/Projects, clone missing repos, and recreate code symlinks.\nIntended for onboarding a new Mac.",
	RunE:  runBootstrap,
}

const maxConcurrentClones = 6

type cloneJob struct {
	project  *project.Project
	repoName string
	repoURL  string
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	if len(projects) == 0 {
		ui.info("No projects found")
		return nil
	}

	// Phase 1: collect missing repos to clone.
	var jobs []cloneJob
	for _, p := range projects {
		if !p.HasCode() {
			continue
		}
		for repoName, repoURL := range p.Repos {
			dest := filepath.Join(p.CodeRoot, repoName)
			if !fsutil.PathExists(dest) && repoURL != "" {
				jobs = append(jobs, cloneJob{project: p, repoName: repoName, repoURL: repoURL})
			}
		}
	}

	if len(jobs) > 0 {
		ui.heading(fmt.Sprintf("Cloning %d repos (max %d parallel)", len(jobs), maxConcurrentClones))
		cloneErrors := parallelCloneJobs(jobs, ui)
		for _, ce := range cloneErrors {
			ui.fail(fmt.Sprintf("%s: clone failed — %v", ce.name, ce.err))
		}
	}

	// Phase 2: ensure code dirs and symlinks.
	ui.heading("Setting up projects")
	hasErrors := false

	for _, p := range projects {
		if !p.HasCode() {
			ui.ok(fmt.Sprintf("%s/%s (no code)", p.Org, p.Name))
			continue
		}

		if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s: create code dir — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}

		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := ensureOrFixCodeSymlink(codeLinkPath, p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s: symlink — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}
		ui.ok(fmt.Sprintf("%s/%s → %s", p.Org, p.Name, tildePath(p.CodeRoot)))
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("bootstrap completed with errors — check output above")
	}
	ui.ok("Bootstrap complete")
	return nil
}

type cloneError struct {
	name string
	err  error
}

func parallelCloneJobs(jobs []cloneJob, ui *UI) []cloneError {
	sem := make(chan struct{}, maxConcurrentClones)
	var mu sync.Mutex
	var errors []cloneError

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j cloneJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ui.info(fmt.Sprintf("Cloning %s/%s/%s", j.project.Org, j.project.Name, j.repoName))
			if err := gitCloneForBootstrap(j.repoURL, filepath.Join(j.project.CodeRoot, j.repoName)); err != nil {
				mu.Lock()
				errors = append(errors, cloneError{name: j.repoName, err: err})
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()

	return errors
}

func gitCloneForBootstrap(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	cmd := newGitCloneCmd(url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// --- nest relink ---

var relinkCmd = &cobra.Command{
	Use:   "relink",
	Short: "Recreate code symlinks for all projects",
	RunE:  runRelink,
}

func runRelink(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	if len(projects) == 0 {
		ui.info("No projects found")
		return nil
	}

	ui.heading("Relinking projects")
	hasErrors := false

	for _, p := range projects {
		if !p.HasCode() {
			continue
		}
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := ensureOrFixCodeSymlink(codeLinkPath, p.CodeRoot); err != nil {
			ui.fail(fmt.Sprintf("%s/%s — %v", p.Org, p.Name, err))
			hasErrors = true
			continue
		}
		ui.ok(fmt.Sprintf("%s/%s → %s", p.Org, p.Name, tildePath(p.CodeRoot)))
	}

	ui.line()
	if hasErrors {
		return fmt.Errorf("relink completed with errors — check output above")
	}
	ui.ok("Relink complete")
	return nil
}

func ensureOrFixCodeSymlink(linkPath, target string) error {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Symlink(target, linkPath)
		}
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		existing, _ := os.Readlink(linkPath)
		if existing == target {
			return nil
		}
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("remove stale symlink: %w", err)
		}
		return os.Symlink(target, linkPath)
	}

	return fmt.Errorf("%s exists but is not a symlink (%s); refusing to overwrite", linkPath, info.Mode().Type())
}

// --- nest list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	if len(projects) == 0 {
		ui.info("No projects found")
		return nil
	}

	w := cmd.OutOrStdout()

	maxName, maxOrg := 4, 3
	for _, p := range projects {
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
		if len(p.Org) > maxOrg {
			maxOrg = len(p.Org)
		}
	}

	format := fmt.Sprintf("  %%-%ds  %%-%ds  %%s\n", maxName, maxOrg)
	fmt.Fprintf(w, format, "NAME", "ORG", "CODE")
	fmt.Fprintf(w, format,
		strings.Repeat("-", maxName),
		strings.Repeat("-", maxOrg),
		strings.Repeat("-", 4))

	for _, p := range projects {
		code := "-"
		if p.HasCode() {
			repos := len(p.Repos)
			if repos > 0 {
				code = fmt.Sprintf("%d repo(s)", repos)
			} else {
				code = "yes"
			}
		}
		fmt.Fprintf(w, format, p.Name, p.Org, code)
	}

	return nil
}

// --- nest doctor ---

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose workspace health",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading("Checking workspace health")
	ui.info(fmt.Sprintf("Storage mode: %s", cfg.Storage.Mode))
	report := doctor.Run(cfg)

	for _, r := range report.Results {
		switch r.Level {
		case doctor.LevelOK:
			ui.ok(r.Message)
		case doctor.LevelInfo:
			ui.info(r.Message)
		case doctor.LevelWarn:
			ui.warn(r.Message)
		case doctor.LevelErr:
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

// --- hive open ---

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
	matches, err := project.FindByQuery(cfg, query)
	if err != nil {
		return err
	}

	switch len(matches) {
	case 0:
		return fmt.Errorf("no project found matching %q", query)
	case 1:
		fmt.Fprint(cmd.OutOrStdout(), matches[0].ProjectRoot)
		return nil
	default:
		msg := fmt.Sprintf("multiple projects match %q:\n", query)
		for _, m := range matches {
			msg += fmt.Sprintf("  %s/%s (%s)\n", m.Org, m.Name, tildePath(m.ProjectRoot))
		}
		return fmt.Errorf("%s", msg)
	}
}

// --- hive version ---

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hive version %s\ncommit: %s\ndate: %s\n", version, commit, date)
	},
}

// --- nest storage ---

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
