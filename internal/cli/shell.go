package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initShellCmd = &cobra.Command{
	Use:       "init-shell <shell>",
	Short:     "Generate shell configuration for h and hi functions",
	Long:      "Print shell functions for the given shell.\nAdd to your shell config:\n  fish: hive init-shell fish | source\n  bash: eval \"$(hive init-shell bash)\"\n  zsh:  eval \"$(hive init-shell zsh)\"",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"fish", "bash", "zsh"},
	RunE:      runInitShell,
}

func runInitShell(cmd *cobra.Command, args []string) error {
	shell := args[0]
	w := cmd.OutOrStdout()

	switch shell {
	case "fish":
		fmt.Fprint(w, fishInit)
	case "bash":
		fmt.Fprint(w, bashInit)
	case "zsh":
		fmt.Fprint(w, zshInit)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", shell)
	}
	return nil
}

const fishInit = `function h
    if test (count $argv) -eq 0
        echo "Usage: h <query>" >&2
        return 1
    end
    set -l dir (hive path $argv)
    and cd $dir
end

function hi
    set -l dir (hive open)
    and cd $dir
end
`

const bashInit = `h() {
    if [ $# -eq 0 ]; then
        echo "Usage: h <query>" >&2
        return 1
    fi
    local dir
    dir="$(hive path "$@")"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

hi() {
    local dir
    dir="$(hive open)"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}
`

const zshInit = `h() {
    if [ $# -eq 0 ]; then
        echo "Usage: h <query>" >&2
        return 1
    fi
    local dir
    dir="$(hive path "$@")"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}

hi() {
    local dir
    dir="$(hive open)"
    if [ $? -eq 0 ]; then
        cd "$dir" || return 1
    fi
}
`
