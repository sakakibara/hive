package project

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/meta"
)

// Project represents a resolved project with absolute paths.
type Project struct {
	Name        string
	Org         string
	ProjectRoot string            // absolute path under ~/Projects/...
	CodeRoot    string            // absolute path under ~/Code/... (empty if no code)
	CodeRel     string            // relative path (e.g., "acme/order-form"), empty if no code
	Repos       map[string]string // repo name -> URL
	Meta        *meta.ProjectMeta
}

// HasCode returns true if the project has an associated code directory.
func (p *Project) HasCode() bool {
	return p.CodeRel != ""
}

// ResolveNew computes the paths for a new project without code.
func ResolveNew(cfg *config.Config, org, name string) *Project {
	org = filepath.Clean(org)
	name = filepath.Clean(name)
	resolved := cfg.Resolved()

	return &Project{
		Name:        name,
		Org:         org,
		ProjectRoot: filepath.Join(resolved.Paths.Projects, org, name),
	}
}

// ResolveNewWithCode computes the paths for a project with a code directory.
func ResolveNewWithCode(cfg *config.Config, org, name string) *Project {
	p := ResolveNew(cfg, org, name)
	codeRel := filepath.Join(org, name)
	resolved := cfg.Resolved()
	p.CodeRoot = filepath.Join(resolved.Paths.Code, codeRel)
	p.CodeRel = codeRel
	return p
}

// RepoNameFromURL extracts the repository name from a git URL.
// e.g., "https://github.com/acme/order-form-api.git" -> "order-form-api"
func RepoNameFromURL(repoURL string) string {
	// Handle SCP-like URLs (git@github.com:user/repo.git)
	if idx := strings.LastIndex(repoURL, ":"); idx != -1 && !strings.Contains(repoURL, "://") {
		repoURL = repoURL[idx+1:]
	}

	// Handle standard URLs
	if u, err := url.Parse(repoURL); err == nil && u.Path != "" {
		repoURL = u.Path
	}

	base := filepath.Base(repoURL)
	return strings.TrimSuffix(base, ".git")
}

// Create sets up a new project's filesystem structure.
func Create(p *Project) error {
	if msg := fsutil.PathConflict(p.ProjectRoot); msg != "" {
		return fmt.Errorf("project root: %s", msg)
	}

	if p.HasCode() {
		if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
			return fmt.Errorf("create code root: %w", err)
		}
	}

	return setupProjectRoot(p)
}

// CloneRepo clones a repo into an existing project's code directory.
// Returns the repo name derived from the URL.
func CloneRepo(cfg *config.Config, p *Project, repoURL string) (string, error) {
	if !p.HasCode() {
		// Upgrade project to have code.
		codeRel := filepath.Join(p.Org, p.Name)
		resolved := cfg.Resolved()
		p.CodeRoot = filepath.Join(resolved.Paths.Code, codeRel)
		p.CodeRel = codeRel

		if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
			return "", fmt.Errorf("create code root: %w", err)
		}

		// Create symlink.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
			return "", fmt.Errorf("create code symlink: %w", err)
		}
	}

	repoName := RepoNameFromURL(repoURL)
	dest := filepath.Join(p.CodeRoot, repoName)

	if msg := fsutil.PathConflict(dest); msg != "" {
		return "", fmt.Errorf("repo destination: %s", msg)
	}

	if err := gitClone(repoURL, dest); err != nil {
		return "", fmt.Errorf("clone %s: %w", repoURL, err)
	}

	// Update metadata.
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	m := p.Meta
	if m == nil {
		m, _ = meta.Read(metaPath)
	}
	if m != nil {
		m.CodeRel = p.CodeRel
		m.AddRepo(repoName, repoURL)
		_ = meta.Write(metaPath, m)
	}

	return repoName, nil
}

// Adopt moves an existing repository into hive's managed structure.
// If the project already exists, the repo is added to it.
func Adopt(cfg *config.Config, org, name, sourcePath string) (*Project, error) {
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path %s is not a directory", sourcePath)
	}

	// Check if the project already exists.
	p := ResolveNewWithCode(cfg, org, name)
	existing := fsutil.IsDir(p.ProjectRoot)

	if existing {
		// Load existing project metadata.
		metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
		m, err := meta.Read(metaPath)
		if err != nil {
			return nil, fmt.Errorf("read existing project: %w", err)
		}
		p.Meta = m
		p.Repos = m.Repos
		if m.HasCode() {
			p.CodeRel = m.CodeRel
			resolved := cfg.Resolved()
			p.CodeRoot = filepath.Join(resolved.Paths.Code, m.CodeRel)
		}
	}

	// Ensure code directory exists.
	if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
		return nil, fmt.Errorf("create code root: %w", err)
	}

	// Move source into code container using source dir name as repo name.
	repoName := filepath.Base(sourcePath)
	dest := filepath.Join(p.CodeRoot, repoName)

	if msg := fsutil.PathConflict(dest); msg != "" {
		return nil, fmt.Errorf("repo destination: %s", msg)
	}

	if err := fsutil.MoveDir(sourcePath, dest); err != nil {
		return nil, fmt.Errorf("move to code root: %w", err)
	}

	// Detect repo URL from the moved directory.
	repoURL := detectRemoteURL(dest)
	if p.Repos == nil {
		p.Repos = make(map[string]string)
	}
	p.Repos[repoName] = repoURL

	if existing {
		// Update existing metadata.
		metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
		m, err := meta.Read(metaPath)
		if err != nil {
			return nil, fmt.Errorf("read metadata: %w", err)
		}
		m.CodeRel = p.CodeRel
		m.AddRepo(repoName, repoURL)
		if err := meta.Write(metaPath, m); err != nil {
			return nil, fmt.Errorf("write metadata: %w", err)
		}

		// Ensure code symlink exists.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
			return nil, fmt.Errorf("create code symlink: %w", err)
		}
	} else {
		if err := setupProjectRoot(p); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// setupProjectRoot creates the project directory, symlink (if code), subdirs, and metadata.
func setupProjectRoot(p *Project) error {
	if err := fsutil.EnsureDir(p.ProjectRoot); err != nil {
		return fmt.Errorf("create project root: %w", err)
	}

	if p.HasCode() {
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
			return fmt.Errorf("create code symlink: %w", err)
		}
	}

	for _, sub := range Subdirs {
		dir := filepath.Join(p.ProjectRoot, sub)
		if err := fsutil.EnsureDir(dir); err != nil {
			return fmt.Errorf("create subdirectory %s: %w", sub, err)
		}
	}

	m := meta.New(p.Name, p.Org, p.CodeRel)
	for name, url := range p.Repos {
		m.AddRepo(name, url)
	}
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	if err := meta.Write(metaPath, m); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// detectRemoteURL returns the git remote URL for a directory, or empty string.
func detectRemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Scan finds all projects under the projects root by looking for .hive.json files.
func Scan(cfg *config.Config) ([]*Project, error) {
	resolved := cfg.Resolved()
	return scanDir(resolved.Paths.Projects, resolved.Paths.Code)
}

// scanDir walks a directory tree looking for .hive.json files and returns the projects found.
func scanDir(root, codeRoot string) ([]*Project, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", root, err)
	}

	if info, err := os.Stat(resolvedRoot); err != nil || !info.IsDir() {
		return nil, nil
	}

	var projects []*Project

	err = filepath.Walk(resolvedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() != meta.FileName {
			return nil
		}

		m, err := meta.Read(path)
		if err != nil {
			return nil
		}

		projRoot := filepath.Dir(path)

		p := &Project{
			Name:        m.Name,
			Org:         m.Org,
			ProjectRoot: projRoot,
			CodeRel:     m.CodeRel,
			Repos:       m.Repos,
			Meta:        m,
		}

		if m.HasCode() {
			p.CodeRoot = filepath.Join(codeRoot, m.CodeRel)
		}

		projects = append(projects, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return projects, nil
}

// Subpaths returns all suffix paths for matching.
func (p *Project) Subpaths() []string {
	rel := filepath.Join(p.Org, p.Name)
	parts := strings.Split(rel, string(filepath.Separator))
	subs := make([]string, len(parts))
	for i := range parts {
		subs[i] = strings.Join(parts[i:], string(filepath.Separator))
	}
	return subs
}

// FindByQuery returns all projects matching the given query using subpath matching.
// Matching is tiered: exact > prefix > substring. Within each tier, smartcase
// applies (all-lowercase query is case-insensitive, mixed-case is exact).
// The highest-priority tier with results wins.
func FindByQuery(cfg *config.Config, query string) ([]*Project, error) {
	all, err := Scan(cfg)
	if err != nil {
		return nil, err
	}
	return filterByQuery(all, query), nil
}

// filterByQuery filters a list of projects by tiered smartcase subpath matching.
func filterByQuery(projects []*Project, query string) []*Project {
	caseSensitive := query != strings.ToLower(query)

	var exact, prefix, substring []*Project
	for _, p := range projects {
		best := matchTier(query, p.Subpaths(), caseSensitive)
		switch best {
		case tierExact:
			exact = append(exact, p)
		case tierPrefix:
			prefix = append(prefix, p)
		case tierSubstring:
			substring = append(substring, p)
		}
	}

	if len(exact) > 0 {
		return exact
	}
	if len(prefix) > 0 {
		return prefix
	}
	return substring
}

type matchTierLevel int

const (
	tierNone      matchTierLevel = iota
	tierSubstring
	tierPrefix
	tierExact
)

// matchTier returns the best match tier for any of the candidate subpaths.
func matchTier(query string, subpaths []string, caseSensitive bool) matchTierLevel {
	best := tierNone
	for _, sub := range subpaths {
		q, s := query, sub
		if !caseSensitive {
			q = strings.ToLower(q)
			s = strings.ToLower(s)
		}

		if q == s {
			return tierExact
		}
		if strings.HasPrefix(s, q) && best < tierPrefix {
			best = tierPrefix
		} else if strings.Contains(s, q) && best < tierSubstring {
			best = tierSubstring
		}
	}
	return best
}

func gitClone(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
