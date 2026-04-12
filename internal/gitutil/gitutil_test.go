package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

// gitEnv returns environment variables for deterministic git commits in tests.
func gitEnv() []string {
	return []string{
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), gitEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "init")

	got := CurrentBranch(repo)
	if got != "main" {
		t.Errorf("CurrentBranch = %q, want %q", got, "main")
	}
}

func TestCurrentBranch_NotGit(t *testing.T) {
	tmp := t.TempDir()
	if got := CurrentBranch(tmp); got != "" {
		t.Errorf("CurrentBranch (not git) = %q, want empty", got)
	}
}

func TestLatestCommitTime(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), gitEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "init")

	got := LatestCommitTime(repo)
	if got.IsZero() {
		t.Error("LatestCommitTime returned zero time, want non-zero")
	}
	if got.Before(time.Now().Add(-1 * time.Minute)) {
		t.Errorf("LatestCommitTime = %v, expected recent time", got)
	}
}

func TestLatestCommitTime_NoCommits(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", out)
	}

	got := LatestCommitTime(repo)
	if !got.IsZero() {
		t.Errorf("LatestCommitTime (no commits) = %v, want zero time", got)
	}
}

func TestStatus(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), gitEnv()...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "commit", "--allow-empty", "-m", "init")

	st := Status(repo)
	if !st.Clean {
		t.Error("Status.Clean = false, want true")
	}
	if st.HasRemote {
		t.Error("Status.HasRemote = true, want false")
	}
	if st.Branch != "main" {
		t.Errorf("Status.Branch = %q, want %q", st.Branch, "main")
	}

	// Add an untracked file to make it dirty.
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	st2 := Status(repo)
	if st2.Clean {
		t.Error("Status.Clean = true after adding untracked file, want false")
	}
}
