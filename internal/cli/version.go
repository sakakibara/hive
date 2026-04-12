package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time:
//
//	go build -ldflags "-X github.com/sakakibara/hive/internal/cli.version=1.0.0"
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hive version %s\ncommit: %s\ndate: %s\n", version, commit, date)
	},
}
