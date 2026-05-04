package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDir_Creates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if !IsDir(dir) {
		t.Error("directory should exist after EnsureDir")
	}
}

func TestEnsureDir_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir on existing dir: %v", err)
	}
}

func TestEnsureDir_ConflictWithFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, []byte("x"), 0644)

	err := EnsureDir(f)
	if err == nil {
		t.Fatal("expected error when path is a file")
	}
}

func TestEnsureSymlink_Creates(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.MkdirAll(target, 0755)
	link := filepath.Join(dir, "link")

	if err := EnsureSymlink(target, link); err != nil {
		t.Fatalf("EnsureSymlink: %v", err)
	}
	if !IsSymlink(link) {
		t.Error("should be a symlink")
	}
	if got := SymlinkTarget(link); got != target {
		t.Errorf("target = %q, want %q", got, target)
	}
}

func TestEnsureSymlink_Idempotent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.MkdirAll(target, 0755)
	link := filepath.Join(dir, "link")

	os.Symlink(target, link)

	if err := EnsureSymlink(target, link); err != nil {
		t.Fatalf("EnsureSymlink idempotent: %v", err)
	}
}

func TestEnsureSymlink_WrongTarget(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "target1")
	target2 := filepath.Join(dir, "target2")
	os.MkdirAll(target1, 0755)
	os.MkdirAll(target2, 0755)
	link := filepath.Join(dir, "link")
	os.Symlink(target1, link)

	err := EnsureSymlink(target2, link)
	if err == nil {
		t.Fatal("expected error for wrong symlink target")
	}
}

func TestEnsureSymlink_ConflictWithDir(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	os.MkdirAll(link, 0755)

	err := EnsureSymlink("/some/target", link)
	if err == nil {
		t.Fatal("expected error when link path is a directory")
	}
}

func TestPathConflict(t *testing.T) {
	dir := t.TempDir()

	// Non-existent path should be fine.
	if msg := PathConflict(filepath.Join(dir, "nope")); msg != "" {
		t.Errorf("expected empty for non-existent path, got: %s", msg)
	}

	// Existing file should conflict.
	f := filepath.Join(dir, "file")
	os.WriteFile(f, []byte("x"), 0644)
	if msg := PathConflict(f); msg == "" {
		t.Error("expected conflict for existing file")
	}
}
