package gitutil

import (
	"os/exec"
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
