package meta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	m := New("myproject", "personal", "personal/myproject")
	m.AddRepo("myproject", "git@github.com:example/myproject.git")

	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	if got.Name != "myproject" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Org != "personal" {
		t.Errorf("org = %q", got.Org)
	}
	if got.CodeRel != "personal/myproject" {
		t.Errorf("code_rel = %q", got.CodeRel)
	}
	if got.Repos["myproject"] != "git@github.com:example/myproject.git" {
		t.Errorf("repos = %v", got.Repos)
	}
	if got.CreatedAt == "" {
		t.Error("created_at should not be empty")
	}
}

func TestWriteAndRead_NoCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	m := New("hr-strategy", "mycompany", "")

	if err := Write(path, m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.CodeRel != "" {
		t.Errorf("code_rel should be empty, got %q", got.CodeRel)
	}
	if got.HasCode() {
		t.Error("HasCode() should be false")
	}
	if got.Repos != nil {
		t.Errorf("repos should be nil, got %v", got.Repos)
	}
}

func TestRead_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	os.WriteFile(path, []byte("{invalid"), 0644)

	_, err := Read(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidate(t *testing.T) {
	m := &ProjectMeta{}
	issues := m.Validate()
	if len(issues) == 0 {
		t.Error("expected validation issues for empty metadata")
	}

	m = New("test", "personal", "personal/test")
	issues = m.Validate()
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}

	m = New("test", "personal", "")
	issues = m.Validate()
	if len(issues) != 0 {
		t.Errorf("expected no issues for no-code project, got: %v", issues)
	}
}

func TestHasCode(t *testing.T) {
	withCode := New("test", "personal", "personal/test")
	if !withCode.HasCode() {
		t.Error("expected HasCode() = true")
	}

	withoutCode := New("test", "personal", "")
	if withoutCode.HasCode() {
		t.Error("expected HasCode() = false")
	}
}

func TestAddRepo(t *testing.T) {
	m := New("test", "acme", "acme/test")
	m.AddRepo("frontend", "https://github.com/acme/frontend.git")
	m.AddRepo("api", "https://github.com/acme/api.git")

	if len(m.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(m.Repos))
	}
	if m.Repos["frontend"] != "https://github.com/acme/frontend.git" {
		t.Errorf("frontend repo = %q", m.Repos["frontend"])
	}
}
