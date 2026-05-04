package lifecycle

import (
	"os"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/fsutil"
	"github.com/sakakibara/hive/internal/project"
)

// Delete permanently removes a project and its code directory.
func Delete(cfg *config.Config, p *project.Project) error {
	resolved := cfg.Resolved()

	// Remove project root.
	if err := os.RemoveAll(p.ProjectRoot); err != nil {
		return err
	}

	// Remove code root if it exists.
	if p.HasCode() && fsutil.IsDir(p.CodeRoot) {
		if err := os.RemoveAll(p.CodeRoot); err != nil {
			return err
		}
	}

	// Clean up empty parent directories.
	fsutil.CleanEmptyParents(p.ProjectRoot, resolved.Paths.Projects)
	if p.HasCode() {
		fsutil.CleanEmptyParents(p.CodeRoot, resolved.Paths.Code)
	}

	return nil
}
