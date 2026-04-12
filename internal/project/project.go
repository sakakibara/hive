package project

import (
	"fmt"
	"net/url"
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
