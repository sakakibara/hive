package cli

import "os/exec"

// newGitCloneCmd creates an exec.Cmd for git clone.
// Separated for testability.
func newGitCloneCmd(url, dest string) *exec.Cmd {
	return exec.Command("git", "clone", url, dest)
}
