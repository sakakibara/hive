package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const repoOwner = "sakakibara"
const repoName = "hive"

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [version]",
	Short: "Upgrade hive to the latest or a specific version",
	Long:  "Download and replace the current hive binary.\nIf no version is specified, the latest release is used.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUpgrade,
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ui := newUI(cmd.OutOrStdout(), cmd.ErrOrStderr())

	if runtime.GOOS != "darwin" {
		return fmt.Errorf("upgrade is only supported on macOS")
	}

	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Determine target version.
	var targetVersion string
	if len(args) == 1 {
		targetVersion = args[0]
		if !strings.HasPrefix(targetVersion, "v") {
			targetVersion = "v" + targetVersion
		}
	} else {
		ui.info("Checking for latest release...")
		v, err := fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("could not determine latest version: %w", err)
		}
		targetVersion = v
	}

	// Check if already on this version.
	if version != "dev" && "v"+version == targetVersion {
		ui.ok(fmt.Sprintf("Already on %s", targetVersion))
		return nil
	}

	// Find the current binary path.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve binary path: %w", err)
	}

	archive := fmt.Sprintf("hive_darwin_%s.tar.gz", arch)
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", repoOwner, repoName, targetVersion, archive)

	ui.info(fmt.Sprintf("Downloading %s for darwin/%s...", targetVersion, arch))

	// Download to a temp file.
	tmpDir, err := os.MkdirTemp("", "hive-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("release %s not found", targetVersion)
		}
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Extract the binary from the tar.gz.
	newBinaryPath := filepath.Join(tmpDir, "hive")
	if err := extractBinaryFromTarGz(resp.Body, newBinaryPath); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	// Replace the current binary.
	if err := replaceBinary(execPath, newBinaryPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	ui.line()
	ui.ok(fmt.Sprintf("Upgraded to %s (%s)", targetVersion, tildePath(execPath)))
	return nil
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in response")
	}
	return release.TagName, nil
}

func extractBinaryFromTarGz(r io.Reader, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("binary 'hive' not found in archive")
		}
		if err != nil {
			return err
		}

		if filepath.Base(header.Name) == "hive" && header.Typeflag == tar.TypeReg {
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			return f.Close()
		}
	}
}

func replaceBinary(oldPath, newPath string) error {
	// Rename the old binary to a backup, move new one in, then remove backup.
	// This is atomic-ish on the same filesystem.
	backupPath := oldPath + ".old"
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := copyFile(newPath, oldPath); err != nil {
		// Attempt to restore backup.
		os.Rename(backupPath, oldPath)
		return fmt.Errorf("install new binary: %w", err)
	}
	os.Remove(backupPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
