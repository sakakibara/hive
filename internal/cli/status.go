package cli

import (
	"fmt"
	"os"

	"github.com/sakakibara/hive/internal/gitutil"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show git status across all projects",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolP("verbose", "v", false, "Show per-project and per-repo details")
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	verbose, _ := cmd.Flags().GetBool("verbose")

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	var totalRepos, dirty, unpushed, clean int

	for _, p := range projects {
		if !p.HasCode() {
			if verbose {
				ui.heading(p.Org + "/" + p.Name)
				ui.info("no code")
			}
			continue
		}

		entries, err := os.ReadDir(p.CodeRoot)
		if err != nil {
			if verbose {
				ui.heading(p.Org + "/" + p.Name)
				ui.warn("cannot read code directory")
			}
			continue
		}

		var repoLines []struct {
			name  string
			label string
			ok    bool
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := p.CodeRoot + "/" + entry.Name()
			if !gitutil.IsGitRepo(dir) {
				continue
			}

			totalRepos++
			st := gitutil.Status(dir)

			var label string
			isOk := true
			switch {
			case !st.Clean && !st.Synced:
				label = "dirty, unpushed"
				dirty++
				unpushed++
				isOk = false
			case !st.Clean:
				label = "dirty"
				dirty++
				isOk = false
			case !st.Synced:
				label = "unpushed"
				unpushed++
				isOk = false
			default:
				label = "clean"
				clean++
			}

			repoLines = append(repoLines, struct {
				name  string
				label string
				ok    bool
			}{entry.Name(), label, isOk})
		}

		if verbose {
			ui.heading(p.Org + "/" + p.Name)
			for _, r := range repoLines {
				if r.ok {
					ui.ok(r.name + " — " + r.label)
				} else {
					ui.warn(r.name + " — " + r.label)
				}
			}
		}
	}

	ui.heading(fmt.Sprintf("%d projects, %d repos", len(projects), totalRepos))
	if dirty > 0 {
		ui.warn(fmt.Sprintf("%d dirty", dirty))
	}
	if unpushed > 0 {
		ui.warn(fmt.Sprintf("%d unpushed", unpushed))
	}
	if clean > 0 {
		ui.ok(fmt.Sprintf("%d clean", clean))
	}

	return nil
}
