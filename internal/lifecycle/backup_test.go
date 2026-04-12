package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/meta"
	"github.com/sakakibara/hive/internal/project"
)

func setupBackupTest(t *testing.T, mode config.StorageMode) (cfg *config.Config, p *project.Project, backupDir string) {
	t.Helper()

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "projects", "acme", "widget")
	codeRoot := filepath.Join(tmp, "code", "acme", "widget")
	backupDir = filepath.Join(tmp, "backups")

	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codeRoot, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a file in project root.
	if err := os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("# Widget"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write metadata.
	m := meta.New("widget", "acme", "acme/widget")
	if err := meta.Write(filepath.Join(projectRoot, ".hive.json"), m); err != nil {
		t.Fatal(err)
	}

	// Write a file in code root.
	repoDir := filepath.Join(codeRoot, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg = &config.Config{
		Paths: config.PathsConfig{
			Projects: filepath.Join(tmp, "projects"),
			Life:     filepath.Join(tmp, "life"),
			Work:     filepath.Join(tmp, "work"),
			Archive:  filepath.Join(tmp, "archive"),
			Code:     filepath.Join(tmp, "code"),
		},
		Storage: config.StorageConfig{
			Mode: mode,
		},
	}

	if mode == config.ModeICloud {
		cfg.Storage.ICloudRoot = tmp
	}

	p = &project.Project{
		Name:        "widget",
		Org:         "acme",
		ProjectRoot: projectRoot,
		CodeRoot:    codeRoot,
		CodeRel:     "acme/widget",
	}

	return cfg, p, backupDir
}

func tarEntries(t *testing.T, path string) []string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

func TestCreateBackup_ICloudMode_CodeOnly(t *testing.T) {
	cfg, p, backupDir := setupBackupTest(t, config.ModeICloud)

	outPath, err := CreateBackup(cfg, p, backupDir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(outPath, "-code.tar.gz") {
		t.Errorf("expected -code.tar.gz suffix, got %s", filepath.Base(outPath))
	}

	entries := tarEntries(t, outPath)

	hasCode := false
	hasProject := false
	for _, e := range entries {
		if strings.HasPrefix(e, "code/") {
			hasCode = true
		}
		if strings.HasPrefix(e, "project/") {
			hasProject = true
		}
	}

	if !hasCode {
		t.Error("expected code/ entries in archive")
	}
	if hasProject {
		t.Error("iCloud mode should not include project/ entries")
	}
}

func TestCreateBackup_LocalMode_Full(t *testing.T) {
	cfg, p, backupDir := setupBackupTest(t, config.ModeLocal)

	outPath, err := CreateBackup(cfg, p, backupDir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(outPath, "-full.tar.gz") {
		t.Errorf("expected -full.tar.gz suffix, got %s", filepath.Base(outPath))
	}

	entries := tarEntries(t, outPath)

	hasCode := false
	hasProject := false
	for _, e := range entries {
		if strings.HasPrefix(e, "code/") {
			hasCode = true
		}
		if strings.HasPrefix(e, "project/") {
			hasProject = true
		}
	}

	if !hasCode {
		t.Error("expected code/ entries in archive")
	}
	if !hasProject {
		t.Error("local mode should include project/ entries")
	}
}

func TestCreateBackup_NoCodeRoot(t *testing.T) {
	cfg, p, _ := setupBackupTest(t, config.ModeICloud)
	p.CodeRoot = ""

	_, err := CreateBackup(cfg, p, t.TempDir())
	if err == nil {
		t.Fatal("expected error for project with no code root")
	}
}

func TestCreateBackup_CodeRootMissing(t *testing.T) {
	cfg, p, _ := setupBackupTest(t, config.ModeICloud)
	p.CodeRoot = filepath.Join(t.TempDir(), "nonexistent")

	_, err := CreateBackup(cfg, p, t.TempDir())
	if err == nil {
		t.Fatal("expected error for nonexistent code root directory")
	}
}
