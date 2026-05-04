package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/meta"
)

func testConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	tmp := t.TempDir()

	projectsDir := filepath.Join(tmp, "Projects")
	codeDir := filepath.Join(tmp, "Code")
	lifeDir := filepath.Join(tmp, "Life")
	workDir := filepath.Join(tmp, "Work")
	archiveDir := filepath.Join(tmp, "Archive")

	for _, d := range []string{projectsDir, codeDir, lifeDir, workDir, archiveDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Projects: projectsDir,
			Life:     lifeDir,
			Work:     workDir,
			Archive:  archiveDir,
			Code:     codeDir,
		},
		Storage: config.StorageConfig{
			Mode: config.ModeLocal,
		},
	}

	return cfg, tmp
}

func TestSync_AddsUntrackedRepo(t *testing.T) {
	cfg, tmp := testConfig(t)
	projectsDir := filepath.Join(tmp, "Projects")
	codeDir := filepath.Join(tmp, "Code")

	// Create a project with metadata but no repos tracked.
	org := "acme"
	name := "widget"
	projDir := filepath.Join(projectsDir, org, name)
	codeRel := filepath.Join(org, name)
	codePath := filepath.Join(codeDir, codeRel)

	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codePath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create code symlink.
	if err := os.Symlink(codePath, filepath.Join(projDir, "code")); err != nil {
		t.Fatal(err)
	}

	// Write metadata with no repos.
	m := meta.New(name, org, codeRel)
	metaPath := filepath.Join(projDir, meta.FileName)
	if err := meta.Write(metaPath, m); err != nil {
		t.Fatal(err)
	}

	// Create an untracked repo dir on disk.
	repoDir := filepath.Join(codePath, "my-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := Sync(cfg)
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	// Should have one repo_added change.
	found := false
	for _, c := range result.Changes {
		if c.RepoName == "my-repo" && c.Action == "repo_added" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected repo_added change for my-repo, got: %+v", result.Changes)
	}

	// Verify metadata was updated.
	updated, err := meta.Read(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Repos["my-repo"]; !ok {
		t.Errorf("expected my-repo in metadata repos, got: %v", updated.Repos)
	}
}

func TestSync_RemovesStaleRepo(t *testing.T) {
	cfg, tmp := testConfig(t)
	projectsDir := filepath.Join(tmp, "Projects")
	codeDir := filepath.Join(tmp, "Code")

	org := "acme"
	name := "gadget"
	projDir := filepath.Join(projectsDir, org, name)
	codeRel := filepath.Join(org, name)
	codePath := filepath.Join(codeDir, codeRel)

	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codePath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create code symlink.
	if err := os.Symlink(codePath, filepath.Join(projDir, "code")); err != nil {
		t.Fatal(err)
	}

	// Write metadata with a repo that does not exist on disk.
	m := meta.New(name, org, codeRel)
	m.AddRepo("ghost-repo", "https://github.com/acme/ghost-repo.git")
	metaPath := filepath.Join(projDir, meta.FileName)
	if err := meta.Write(metaPath, m); err != nil {
		t.Fatal(err)
	}

	result, err := Sync(cfg)
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	// Should have one repo_removed change.
	found := false
	for _, c := range result.Changes {
		if c.RepoName == "ghost-repo" && c.Action == "repo_removed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected repo_removed change for ghost-repo, got: %+v", result.Changes)
	}

	// Verify metadata was updated.
	updated, err := meta.Read(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Repos["ghost-repo"]; ok {
		t.Errorf("expected ghost-repo to be removed from metadata, got: %v", updated.Repos)
	}
}

func TestSync_FixesBrokenSymlink(t *testing.T) {
	cfg, tmp := testConfig(t)
	projectsDir := filepath.Join(tmp, "Projects")
	codeDir := filepath.Join(tmp, "Code")

	org := "acme"
	name := "broken"
	projDir := filepath.Join(projectsDir, org, name)
	codeRel := filepath.Join(org, name)
	codePath := filepath.Join(codeDir, codeRel)

	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codePath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create code symlink pointing to wrong target.
	wrongTarget := filepath.Join(tmp, "wrong-target")
	if err := os.MkdirAll(wrongTarget, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wrongTarget, filepath.Join(projDir, "code")); err != nil {
		t.Fatal(err)
	}

	// Write metadata.
	m := meta.New(name, org, codeRel)
	metaPath := filepath.Join(projDir, meta.FileName)
	if err := meta.Write(metaPath, m); err != nil {
		t.Fatal(err)
	}

	result, err := Sync(cfg)
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	// Should have a symlink fix with action "fixed".
	found := false
	for _, fix := range result.SymlinkFixes {
		if fix.Project == "acme/broken" && fix.Action == "fixed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected symlink fix with action 'fixed' for acme/broken, got: %+v", result.SymlinkFixes)
	}

	// Verify symlink now points to correct target.
	actual, err := os.Readlink(filepath.Join(projDir, "code"))
	if err != nil {
		t.Fatal(err)
	}
	if actual != codePath {
		t.Errorf("symlink target = %s, want %s", actual, codePath)
	}
}
