package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/meta"
	"github.com/sakakibara/hive/internal/project"
)

func setupArchiveTest(t *testing.T) (*config.Config, *project.Project) {
	t.Helper()

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "projects", "acme", "widget")
	codeRoot := filepath.Join(tmp, "code", "acme", "widget")

	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codeRoot, 0755); err != nil {
		t.Fatal(err)
	}

	// Create code symlink.
	if err := os.Symlink(codeRoot, filepath.Join(projectRoot, "code")); err != nil {
		t.Fatal(err)
	}

	// Create a repo directory in code.
	repoDir := filepath.Join(codeRoot, "api")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another repo directory.
	frontendDir := filepath.Join(codeRoot, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write metadata.
	m := meta.New("widget", "acme", "acme/widget")
	if err := meta.Write(filepath.Join(projectRoot, ".hive.json"), m); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Projects: filepath.Join(tmp, "projects"),
			Life:     filepath.Join(tmp, "life"),
			Work:     filepath.Join(tmp, "work"),
			Archive:  filepath.Join(tmp, "archive"),
			Code:     filepath.Join(tmp, "code"),
		},
		Storage: config.StorageConfig{
			Mode: config.ModeLocal,
		},
	}

	p := &project.Project{
		Name:        "widget",
		Org:         "acme",
		ProjectRoot: projectRoot,
		CodeRoot:    codeRoot,
		CodeRel:     "acme/widget",
	}

	return cfg, p
}

func TestArchive_PreservesEverythingByDefault(t *testing.T) {
	cfg, p := setupArchiveTest(t)

	result, err := Archive(cfg, p, false)
	if err != nil {
		t.Fatal(err)
	}

	// All repos should be preserved (not deleted).
	for _, r := range result.Repos {
		if r.Deleted {
			t.Errorf("repo %s was deleted, expected preserved", r.Name)
		}
	}

	// Verify repos are in archive/code.
	resolved := cfg.Resolved()
	archiveCodeDir := filepath.Join(resolved.Paths.Archive, "code", "acme", "widget")
	for _, name := range []string{"api", "frontend"} {
		repoPath := filepath.Join(archiveCodeDir, name)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			t.Errorf("expected repo %s to exist in archive at %s", name, repoPath)
		}
	}

	// Verify project root moved to archive.
	archiveProjectDir := filepath.Join(resolved.Paths.Archive, "projects", "acme", "widget")
	if _, err := os.Stat(archiveProjectDir); os.IsNotExist(err) {
		t.Error("expected project root to exist in archive")
	}

	// Verify original locations are gone.
	if _, err := os.Stat(p.ProjectRoot); !os.IsNotExist(err) {
		t.Error("expected original project root to be removed")
	}
}

func TestArchive_PruneDeletesSpecifiedRepos(t *testing.T) {
	cfg, p := setupArchiveTest(t)

	result, err := Archive(cfg, p, true, "api")
	if err != nil {
		t.Fatal(err)
	}

	// Check results.
	found := make(map[string]bool)
	for _, r := range result.Repos {
		found[r.Name] = r.Deleted
	}

	if !found["api"] {
		t.Error("expected api to be marked as deleted")
	}
	if found["frontend"] {
		t.Error("expected frontend to be preserved, not deleted")
	}

	// Verify frontend is in archive/code but api is not.
	resolved := cfg.Resolved()
	archiveCodeDir := filepath.Join(resolved.Paths.Archive, "code", "acme", "widget")

	frontendPath := filepath.Join(archiveCodeDir, "frontend")
	if _, err := os.Stat(frontendPath); os.IsNotExist(err) {
		t.Error("expected frontend to exist in archive")
	}

	apiPath := filepath.Join(archiveCodeDir, "api")
	if _, err := os.Stat(apiPath); !os.IsNotExist(err) {
		t.Error("expected api to be deleted, not in archive")
	}
}

func TestFindSafeToDeleteRepos_NoSafe(t *testing.T) {
	tmp := t.TempDir()
	codeRoot := filepath.Join(tmp, "code", "acme", "widget")

	// Create a non-git directory (no .git).
	repoDir := filepath.Join(codeRoot, "my-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &project.Project{
		Name:     "widget",
		Org:      "acme",
		CodeRoot: codeRoot,
		CodeRel:  "acme/widget",
	}

	safe := FindSafeToDeleteRepos(p)
	if len(safe) != 0 {
		t.Errorf("expected no safe repos, got %v", safe)
	}
}
