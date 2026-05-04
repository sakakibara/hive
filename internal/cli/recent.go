package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/sakakibara/hive/internal/gitutil"
	"github.com/sakakibara/hive/internal/project"
	"github.com/spf13/cobra"
)

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List recently active projects",
	RunE:  runRecent,
}

func init() {
	recentCmd.Flags().IntP("count", "n", 10, "Number of projects to show")
	recentCmd.Flags().Bool("all", false, "Show all projects")
}

func runRecent(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	count, _ := cmd.Flags().GetInt("count")
	showAll, _ := cmd.Flags().GetBool("all")

	projects, err := project.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scan projects: %w", err)
	}

	type entry struct {
		label string
		t     time.Time
	}

	var entries []entry

	for _, p := range projects {
		var latest time.Time

		if p.HasCode() {
			dirs, err := os.ReadDir(p.CodeRoot)
			if err == nil {
				for _, d := range dirs {
					if !d.IsDir() {
						continue
					}
					dir := p.CodeRoot + "/" + d.Name()
					if !gitutil.IsGitRepo(dir) {
						continue
					}
					t := gitutil.LatestCommitTime(dir)
					if t.After(latest) {
						latest = t
					}
				}
			}
		}

		if latest.IsZero() && p.Meta != nil && p.Meta.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, p.Meta.CreatedAt)
			if err == nil {
				latest = t
			}
		}

		entries = append(entries, entry{
			label: p.Org + "/" + p.Name,
			t:     latest,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].t.After(entries[j].t)
	})

	limit := count
	if showAll || limit > len(entries) {
		limit = len(entries)
	}

	w := cmd.OutOrStdout()
	for _, e := range entries[:limit] {
		age := relativeTime(e.t)
		fmt.Fprintf(w, "  %-30s %s\n", e.label, age)
	}

	return nil
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}
