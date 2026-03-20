package cli

import (
	"fmt"

	"github.com/sakakibara/hive/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose workspace health",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	ui.heading("Checking workspace health")
	ui.info(fmt.Sprintf("Storage mode: %s", cfg.Storage.Mode))
	report := doctor.Run(cfg)

	for _, r := range report.Results {
		switch r.Level {
		case doctor.LevelOK:
			ui.ok(r.Message)
		case doctor.LevelInfo:
			ui.info(r.Message)
		case doctor.LevelWarn:
			ui.warn(r.Message)
		case doctor.LevelErr:
			ui.fail(r.Message)
		}
	}

	ui.line()
	if report.HasErrors() {
		return fmt.Errorf("workspace has errors — see above for details")
	}
	ui.ok("Workspace is healthy")
	return nil
}
