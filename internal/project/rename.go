package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/meta"
)

// Rename changes a project's name, moving its directories and updating metadata.
func Rename(cfg *config.Config, p *Project, newName string) error {
	if newName == p.Name {
		return fmt.Errorf("new name is the same as current name")
	}

	resolved := cfg.Resolved()
	newProjectRoot := filepath.Join(resolved.Paths.Projects, p.Org, newName)

	if msg := fsutil.PathConflict(newProjectRoot); msg != "" {
		return fmt.Errorf("project destination: %s", msg)
	}

	// Move code directory first if it exists.
	if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
		newCodeRel := filepath.Join(p.Org, newName)
		newCodeRoot := filepath.Join(resolved.Paths.Code, newCodeRel)

		if msg := fsutil.PathConflict(newCodeRoot); msg != "" {
			return fmt.Errorf("code destination: %s", msg)
		}

		if err := os.MkdirAll(filepath.Dir(newCodeRoot), 0755); err != nil {
			return fmt.Errorf("create code parent: %w", err)
		}
		if err := fsutil.MoveDir(p.CodeRoot, newCodeRoot); err != nil {
			return fmt.Errorf("move code directory: %w", err)
		}
		fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)

		// Remove old code symlink before moving project root.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if fsutil.IsSymlink(codeLinkPath) {
			os.Remove(codeLinkPath)
		}

		p.CodeRel = newCodeRel
		p.CodeRoot = newCodeRoot
	}

	// Move project root.
	if err := os.MkdirAll(filepath.Dir(newProjectRoot), 0755); err != nil {
		return fmt.Errorf("create project parent: %w", err)
	}
	if err := fsutil.MoveDir(p.ProjectRoot, newProjectRoot); err != nil {
		return fmt.Errorf("move project directory: %w", err)
	}
	fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)

	p.ProjectRoot = newProjectRoot
	p.Name = newName

	// Recreate code symlink.
	if p.HasCode() {
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
			return fmt.Errorf("recreate code symlink: %w", err)
		}
	}

	// Update metadata.
	return updateMeta(p)
}

// RenameOrg changes the org for all projects under the old org name.
func RenameOrg(cfg *config.Config, oldOrg, newOrg string) ([]*Project, error) {
	all, err := Scan(cfg)
	if err != nil {
		return nil, err
	}

	var targets []*Project
	for _, p := range all {
		if p.Org == oldOrg {
			targets = append(targets, p)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no projects found under org %q", oldOrg)
	}

	resolved := cfg.Resolved()

	for _, p := range targets {
		newProjectRoot := filepath.Join(resolved.Paths.Projects, newOrg, p.Name)

		if msg := fsutil.PathConflict(newProjectRoot); msg != "" {
			return nil, fmt.Errorf("project %s destination: %s", p.Name, msg)
		}

		if p.HasCode() {
			newCodeRoot := filepath.Join(resolved.Paths.Code, newOrg, p.Name)
			if msg := fsutil.PathConflict(newCodeRoot); msg != "" {
				return nil, fmt.Errorf("project %s code destination: %s", p.Name, msg)
			}
		}
	}

	// All conflict checks passed; proceed with moves.
	for _, p := range targets {
		newProjectRoot := filepath.Join(resolved.Paths.Projects, newOrg, p.Name)

		// Move code.
		if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
			newCodeRel := filepath.Join(newOrg, p.Name)
			newCodeRoot := filepath.Join(resolved.Paths.Code, newCodeRel)

			if err := os.MkdirAll(filepath.Dir(newCodeRoot), 0755); err != nil {
				return nil, fmt.Errorf("create code parent for %s: %w", p.Name, err)
			}
			if err := fsutil.MoveDir(p.CodeRoot, newCodeRoot); err != nil {
				return nil, fmt.Errorf("move code for %s: %w", p.Name, err)
			}
			fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)

			codeLinkPath := filepath.Join(p.ProjectRoot, "code")
			if fsutil.IsSymlink(codeLinkPath) {
				os.Remove(codeLinkPath)
			}

			p.CodeRel = newCodeRel
			p.CodeRoot = newCodeRoot
		}

		// Move project root.
		if err := os.MkdirAll(filepath.Dir(newProjectRoot), 0755); err != nil {
			return nil, fmt.Errorf("create project parent for %s: %w", p.Name, err)
		}
		if err := fsutil.MoveDir(p.ProjectRoot, newProjectRoot); err != nil {
			return nil, fmt.Errorf("move project %s: %w", p.Name, err)
		}
		fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)

		p.ProjectRoot = newProjectRoot
		p.Org = newOrg

		// Recreate code symlink.
		if p.HasCode() {
			codeLinkPath := filepath.Join(p.ProjectRoot, "code")
			if err := fsutil.EnsureSymlink(p.CodeRoot, codeLinkPath); err != nil {
				return nil, fmt.Errorf("recreate code symlink for %s: %w", p.Name, err)
			}
		}

		if err := updateMeta(p); err != nil {
			return nil, fmt.Errorf("update metadata for %s: %w", p.Name, err)
		}
	}

	return targets, nil
}

// updateMeta writes the current project state back to .hive.json.
func updateMeta(p *Project) error {
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	m, err := meta.Read(metaPath)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}
	m.Name = p.Name
	m.Org = p.Org
	m.CodeRel = p.CodeRel
	return meta.Write(metaPath, m)
}
