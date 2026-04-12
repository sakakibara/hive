package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/project"
)

// CreateBackup creates a .tar.gz backup of a project.
// In iCloud mode, only the code directory is backed up.
// In local mode, both code and project directories are included.
func CreateBackup(cfg *config.Config, p *project.Project, backupDir string) (string, error) {
	if p.CodeRoot == "" {
		return "", fmt.Errorf("project %s/%s has no code directory — nothing to back up", p.Org, p.Name)
	}

	if _, err := os.Stat(p.CodeRoot); os.IsNotExist(err) {
		return "", fmt.Errorf("code directory does not exist: %s", p.CodeRoot)
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	now := time.Now()
	timestamp := now.Format("2006-01-02T1504")

	var suffix string
	if cfg.IsICloud() {
		suffix = "code"
	} else {
		suffix = "full"
	}

	filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", p.Org, p.Name, timestamp, suffix)
	outPath := filepath.Join(backupDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := addDirToTar(tw, p.CodeRoot, "code"); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("archive code directory: %w", err)
	}

	if !cfg.IsICloud() {
		if err := addDirToTar(tw, p.ProjectRoot, "project"); err != nil {
			os.Remove(outPath)
			return "", fmt.Errorf("archive project directory: %w", err)
		}
	}

	return outPath, nil
}

func addDirToTar(tw *tar.Writer, srcDir string, prefix string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		name := filepath.Join(prefix, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}
