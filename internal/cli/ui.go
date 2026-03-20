package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// UI provides consistent formatted output for CLI commands.
type UI struct {
	w   io.Writer
	err io.Writer
}

func newUI(w, ew io.Writer) *UI {
	return &UI{w: w, err: ew}
}

func (u *UI) heading(msg string)  { fmt.Fprintf(u.w, "\n%s\n", msg) }
func (u *UI) ok(msg string)       { fmt.Fprintf(u.w, "  \033[32m✔\033[0m %s\n", msg) }
func (u *UI) fail(msg string)     { fmt.Fprintf(u.w, "  \033[31m✖\033[0m %s\n", msg) }
func (u *UI) info(msg string)     { fmt.Fprintf(u.w, "  \033[34mℹ\033[0m %s\n", msg) }
func (u *UI) warn(msg string)     { fmt.Fprintf(u.w, "  \033[33m!\033[0m %s\n", msg) }
func (u *UI) hint(msg string)     { fmt.Fprintf(u.w, "    %s\n", msg) }
func (u *UI) line()               { fmt.Fprintln(u.w) }

// descExisting returns a human-readable description of what exists at the given path.
func descExisting(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return "unknown"
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.IsDir():
		return "directory"
	default:
		return "regular file"
	}
}

// tildePath replaces the home directory prefix with ~ for display.
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}
