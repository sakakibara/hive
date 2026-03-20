package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectRemoteURL runs "git remote get-url origin" in dir and returns the URL.
// Returns "" if dir is not a git repo, has no origin remote, or git is not installed.
func DetectRemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsGitRepo returns true if the directory contains a .git subdirectory or file.
func IsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// IsClean returns true if the git working tree has no uncommitted changes
// (no modified, staged, or untracked files).
func IsClean(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == ""
}

// IsRemoteSynced returns true if the current branch is not ahead of its remote tracking branch.
func IsRemoteSynced(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "@{upstream}..HEAD")
	out, err := cmd.Output()
	if err != nil {
		// No upstream configured — not synced.
		return false
	}
	return strings.TrimSpace(string(out)) == "0"
}

// SafeToDelete returns true if the repo is a clean git repo with a remote and all commits pushed.
func SafeToDelete(dir string) bool {
	return IsGitRepo(dir) &&
		DetectRemoteURL(dir) != "" &&
		IsClean(dir) &&
		IsRemoteSynced(dir)
}
