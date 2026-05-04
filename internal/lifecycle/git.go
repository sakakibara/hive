package lifecycle

import "os/exec"

func newGitCloneCmd(url, dest string) *exec.Cmd {
	return exec.Command("git", "clone", url, dest)
}
