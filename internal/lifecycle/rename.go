package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/meta"
	"github.com/sakakibara/hive/internal/project"
)

// Rename renames a project within the same org.
// It moves the code directory, project directory, recreates the symlink,
// and updates the metadata.
func Rename(cfg *config.Config, p *project.Project, newName string) error {
	resolved := cfg.Resolved()

	newProjectRoot := filepath.Join(resolved.Paths.Projects, p.Org, newName)
	if msg := fsutil.PathConflict(newProjectRoot); msg != "" {
		return fmt.Errorf("new project location: %s", msg)
	}

	// Move code directory if it exists.
	var newCodeRoot string
	var newCodeRel string
	if p.HasCode() {
		newCodeRel = filepath.Join(p.Org, newName)
		newCodeRoot = filepath.Join(resolved.Paths.Code, newCodeRel)

		if msg := fsutil.PathConflict(newCodeRoot); msg != "" {
			return fmt.Errorf("new code location: %s", msg)
		}

		if fsutil.IsDir(p.CodeRoot) {
			if err := os.MkdirAll(filepath.Dir(newCodeRoot), 0755); err != nil {
				return fmt.Errorf("create code parent: %w", err)
			}
			if err := fsutil.MoveDir(p.CodeRoot, newCodeRoot); err != nil {
				return fmt.Errorf("move code directory: %w", err)
			}
			fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)
		}
	}

	// Remove old code symlink before moving project.
	codeLinkPath := filepath.Join(p.ProjectRoot, "code")
	if fsutil.IsSymlink(codeLinkPath) {
		os.Remove(codeLinkPath)
	}

	// Move project directory.
	if err := os.MkdirAll(filepath.Dir(newProjectRoot), 0755); err != nil {
		return fmt.Errorf("create project parent: %w", err)
	}
	if err := fsutil.MoveDir(p.ProjectRoot, newProjectRoot); err != nil {
		return fmt.Errorf("move project directory: %w", err)
	}
	fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)

	// Recreate code symlink with new target.
	if p.HasCode() && newCodeRoot != "" {
		newCodeLink := filepath.Join(newProjectRoot, "code")
		if err := os.Symlink(newCodeRoot, newCodeLink); err != nil {
			return fmt.Errorf("create code symlink: %w", err)
		}
	}

	// Update metadata.
	p.Name = newName
	p.ProjectRoot = newProjectRoot
	if p.HasCode() {
		p.CodeRoot = newCodeRoot
		p.CodeRel = newCodeRel
	}

	return updateMeta(p)
}

// RenameOrg renames an org, moving all its projects.
// It pre-validates all moves before executing any.
func RenameOrg(cfg *config.Config, oldOrg, newOrg string) error {
	resolved := cfg.Resolved()

	// Find all projects in the old org.
	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	var orgProjects []*project.Project
	for _, p := range projects {
		if p.Org == oldOrg {
			orgProjects = append(orgProjects, p)
		}
	}

	if len(orgProjects) == 0 {
		return fmt.Errorf("no projects found in org %q", oldOrg)
	}

	// Pre-validate all destination paths.
	for _, p := range orgProjects {
		newProjectRoot := filepath.Join(resolved.Paths.Projects, newOrg, p.Name)
		if msg := fsutil.PathConflict(newProjectRoot); msg != "" {
			return fmt.Errorf("project %s: new location: %s", p.Name, msg)
		}
		if p.HasCode() {
			newCodeRoot := filepath.Join(resolved.Paths.Code, newOrg, p.Name)
			if msg := fsutil.PathConflict(newCodeRoot); msg != "" {
				return fmt.Errorf("project %s: new code location: %s", p.Name, msg)
			}
		}
	}

	// Execute all moves.
	for _, p := range orgProjects {
		newProjectRoot := filepath.Join(resolved.Paths.Projects, newOrg, p.Name)
		newCodeRel := filepath.Join(newOrg, p.Name)
		newCodeRoot := filepath.Join(resolved.Paths.Code, newCodeRel)

		// Move code directory.
		if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
			if err := os.MkdirAll(filepath.Dir(newCodeRoot), 0755); err != nil {
				return fmt.Errorf("create code parent for %s: %w", p.Name, err)
			}
			if err := fsutil.MoveDir(p.CodeRoot, newCodeRoot); err != nil {
				return fmt.Errorf("move code for %s: %w", p.Name, err)
			}
		}

		// Remove old code symlink.
		codeLinkPath := filepath.Join(p.ProjectRoot, "code")
		if fsutil.IsSymlink(codeLinkPath) {
			os.Remove(codeLinkPath)
		}

		// Move project directory.
		if err := os.MkdirAll(filepath.Dir(newProjectRoot), 0755); err != nil {
			return fmt.Errorf("create project parent for %s: %w", p.Name, err)
		}
		if err := fsutil.MoveDir(p.ProjectRoot, newProjectRoot); err != nil {
			return fmt.Errorf("move project %s: %w", p.Name, err)
		}

		// Recreate code symlink.
		if p.HasCode() {
			newCodeLink := filepath.Join(newProjectRoot, "code")
			if err := os.Symlink(newCodeRoot, newCodeLink); err != nil {
				return fmt.Errorf("create code symlink for %s: %w", p.Name, err)
			}
		}

		// Update project fields and metadata.
		p.Org = newOrg
		p.ProjectRoot = newProjectRoot
		if p.HasCode() {
			p.CodeRoot = newCodeRoot
			p.CodeRel = newCodeRel
		}

		if err := updateMeta(p); err != nil {
			return fmt.Errorf("update metadata for %s: %w", p.Name, err)
		}
	}

	// Clean up empty old org directories.
	oldProjectOrgDir := filepath.Join(resolved.Paths.Projects, oldOrg)
	if fsutil.IsEmptyDir(oldProjectOrgDir) {
		os.Remove(oldProjectOrgDir)
	}
	oldCodeOrgDir := filepath.Join(resolved.Paths.Code, oldOrg)
	if fsutil.IsEmptyDir(oldCodeOrgDir) {
		os.Remove(oldCodeOrgDir)
	}

	return nil
}

// updateMeta reads, updates, and writes the .hive.json for a project.
func updateMeta(p *project.Project) error {
	metaPath := filepath.Join(p.ProjectRoot, meta.FileName)
	m, err := meta.Read(metaPath)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}

	m.Name = p.Name
	m.Org = p.Org
	m.CodeRel = p.CodeRel

	if err := meta.Write(metaPath, m); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	return nil
}
