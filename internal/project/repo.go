package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/meta"
)

// EnsureCode upgrades a project to have a code directory and symlink.
// If the project already has code, it is a no-op.
func EnsureCode(cfg *config.Config, p *Project) error {
	if p.HasCode() {
		return nil
	}

	codeRel := filepath.Join(p.Org, p.Name)
	resolved := cfg.Resolved()
	p.CodeRoot = filepath.Join(resolved.Paths.Code, codeRel)
	p.CodeRel = codeRel

	if err := fsutil.EnsureDir(p.CodeRoot); err != nil {
		return fmt.Errorf("create code root: %w", err)
	}

	codeLinkPath := filepath.Join(p.ProjectRoot, "code")
	if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
		return fmt.Errorf("create code symlink: %w", err)
	}

	// Update metadata.
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	m := p.Meta
	if m == nil {
		m, _ = meta.Read(metaPath)
	}
	if m != nil {
		m.CodeRel = p.CodeRel
		_ = meta.Write(metaPath, m)
	}

	return nil
}

// CloneRepo clones a repo into an existing project's code directory.
// Returns the repo name derived from the URL.
func CloneRepo(cfg *config.Config, p *Project, repoURL string) (string, error) {
	if err := EnsureCode(cfg, p); err != nil {
		return "", err
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

// detectRemoteURL returns the git remote URL for a directory, or empty string.
func detectRemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
