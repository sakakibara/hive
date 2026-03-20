package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectRemoteURL(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")

	// Set up a git repo with an origin remote.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	run("git", "init")
	run("git", "remote", "add", "origin", "https://example.com/repo.git")

	got := DetectRemoteURL(repo)
	if got != "https://example.com/repo.git" {
		t.Errorf("DetectRemoteURL = %q, want https://example.com/repo.git", got)
	}
}

func TestDetectRemoteURL_NoRemote(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(repo, 0755)
	exec.Command("git", "init", repo).Run()

	if got := DetectRemoteURL(repo); got != "" {
		t.Errorf("DetectRemoteURL (no remote) = %q, want empty", got)
	}
}

func TestDetectRemoteURL_NotGitDir(t *testing.T) {
	tmp := t.TempDir()
	if got := DetectRemoteURL(tmp); got != "" {
		t.Errorf("DetectRemoteURL (not git) = %q, want empty", got)
	}
}
