package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/workspace"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:       "init <shell>",
	Short:     "Print shell integration code (functions + completion)",
	Long: `Print everything needed to integrate hive with your shell:
  * h and hi convenience functions (cd to a project)
  * completion for h
  * completion for the hive binary itself

Add to your shell config:
  fish: hive init fish | source
  bash: eval "$(hive init bash)"
  zsh:  eval "$(hive init zsh)"`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"fish", "bash", "zsh"},
	RunE:      runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	shell := args[0]

	shim, err := workspace.ShellInit(shell)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), shim)

	switch shell {
	case "fish":
		return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
	case "bash":
		return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
	case "zsh":
		return rootCmd.GenZshCompletion(cmd.OutOrStdout())
	}
	return nil
}
