package workspace

import (
	"strings"
	"testing"
)

func TestShellInit_UnsupportedShell(t *testing.T) {
	if _, err := ShellInit("nushell"); err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

func TestShellInit_ZshUsesCompdefNotComplete(t *testing.T) {
	out, err := ShellInit("zsh")
	if err != nil {
		t.Fatal(err)
	}
	// `complete -F` is a bash builtin and produces "command not found"
	// when sourced into zsh. zsh completion must register via compdef.
	if strings.Contains(out, "complete -F") {
		t.Errorf("zsh init emits bash-style `complete -F` (broken in zsh)")
	}
	if !strings.Contains(out, "compdef") {
		t.Errorf("zsh init missing compdef registration")
	}
	if strings.Contains(out, "COMPREPLY") {
		t.Errorf("zsh init emits bash COMPREPLY array (zsh uses compadd)")
	}
}

func TestShellInit_BashKeepsCompleteForm(t *testing.T) {
	out, err := ShellInit("bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "complete -F") {
		t.Errorf("bash init lost its `complete -F` registration")
	}
}

func TestShellInit_FishUsesCompleteCFlag(t *testing.T) {
	out, err := ShellInit("fish")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "complete -c h") {
		t.Errorf("fish init lost its `complete -c` registration")
	}
}
