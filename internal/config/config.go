package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// StorageMode represents how nest manages filesystem layout.
type StorageMode string

const (
	ModeICloud StorageMode = "icloud"
	ModeLocal  StorageMode = "local"
)

// Config holds all nest configuration.
type Config struct {
	Paths   PathsConfig   `toml:"paths"`
	Storage StorageConfig `toml:"storage"`
}

// PathsConfig holds filesystem path configuration.
// These are the canonical paths that all commands use.
type PathsConfig struct {
	Projects string `toml:"projects"`
	Life     string `toml:"life"`
	Work     string `toml:"work"`
	Archive  string `toml:"archive"`
	Code     string `toml:"code"`
}

// StorageConfig holds storage backend configuration.
type StorageConfig struct {
	Mode       StorageMode `toml:"mode"`
	ICloudRoot string      `toml:"icloud_root"`
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hive", "config.toml")
}

// DefaultICloudRoot returns the default iCloud workspace root path.
func DefaultICloudRoot() string {
	return "~/Library/Mobile Documents/com~apple~CloudDocs/workspace"
}

// Exists returns true if the config file exists at the default path.
func Exists() bool {
	_, err := os.Stat(DefaultConfigPath())
	return err == nil
}

// Load reads the config from the default path. Returns an error if the file does not exist.
func Load() (*Config, error) {
	return LoadFrom(DefaultConfigPath())
}

// LoadFrom reads the config from the given path.
func LoadFrom(path string) (*Config, error) {
	expandedPath := ExpandPath(path)
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that the config has all required fields.
func (c *Config) Validate() error {
	if c.Storage.Mode == "" {
		return fmt.Errorf("config: storage.mode is required (must be %q or %q)", ModeICloud, ModeLocal)
	}
	if c.Storage.Mode != ModeICloud && c.Storage.Mode != ModeLocal {
		return fmt.Errorf("config: storage.mode must be %q or %q, got %q", ModeICloud, ModeLocal, c.Storage.Mode)
	}
	if c.Storage.Mode == ModeICloud && c.Storage.ICloudRoot == "" {
		return fmt.Errorf("config: storage.icloud_root is required when mode is %q", ModeICloud)
	}
	if c.Paths.Projects == "" {
		return fmt.Errorf("config: paths.projects is required")
	}
	if c.Paths.Life == "" {
		return fmt.Errorf("config: paths.life is required")
	}
	if c.Paths.Work == "" {
		return fmt.Errorf("config: paths.work is required")
	}
	if c.Paths.Archive == "" {
		return fmt.Errorf("config: paths.archive is required")
	}
	if c.Paths.Code == "" {
		return fmt.Errorf("config: paths.code is required")
	}
	return nil
}

// Save writes the config to the default path.
func (c *Config) Save() error {
	return c.SaveTo(DefaultConfigPath())
}

// SaveTo writes the config to the given path.
func (c *Config) SaveTo(path string) error {
	expandedPath := ExpandPath(path)
	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.Create(expandedPath)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(c)
}

// DetectAndCreate performs init-time auto-detection and writes a new config.
// This must ONLY be called when no config file exists.
func DetectAndCreate() (*Config, error) {
	if Exists() {
		return nil, fmt.Errorf("config already exists at %s", DefaultConfigPath())
	}

	icloudRoot := ExpandPath(DefaultICloudRoot())
	mode := ModeLocal
	if isDir(icloudRoot) {
		mode = ModeICloud
	}

	cfg := newConfigForMode(mode)
	if err := cfg.Save(); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	return cfg, nil
}

// newConfigForMode creates a Config with appropriate defaults for the given mode.
func newConfigForMode(mode StorageMode) *Config {
	cfg := &Config{
		Storage: StorageConfig{
			Mode: mode,
		},
	}

	switch mode {
	case ModeICloud:
		cfg.Storage.ICloudRoot = DefaultICloudRoot()
		cfg.Paths = PathsConfig{
			Projects: "~/Projects",
			Life:     "~/Life",
			Work:     "~/Work",
			Archive:  "~/Archive",
			Code:     "~/Code",
		}
	case ModeLocal:
		cfg.Paths = PathsConfig{
			Projects: "~/Projects",
			Life:     "~/Life",
			Work:     "~/Work",
			Archive:  "~/Archive",
			Code:     "~/Code",
		}
	}

	return cfg
}

// ExpandPath replaces a leading ~ with the user's home directory.
func ExpandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// Resolved returns a copy of the config with all paths expanded.
func (c *Config) Resolved() *Config {
	return &Config{
		Paths: PathsConfig{
			Projects: ExpandPath(c.Paths.Projects),
			Life:     ExpandPath(c.Paths.Life),
			Work:     ExpandPath(c.Paths.Work),
			Archive:  ExpandPath(c.Paths.Archive),
			Code:     ExpandPath(c.Paths.Code),
		},
		Storage: StorageConfig{
			Mode:       c.Storage.Mode,
			ICloudRoot: ExpandPath(c.Storage.ICloudRoot),
		},
	}
}

// IsICloud returns true if the storage mode is iCloud.
func (c *Config) IsICloud() bool {
	return c.Storage.Mode == ModeICloud
}

// ICloudDir returns the resolved path to a subdirectory within the iCloud workspace.
// Only valid when mode is icloud.
func (c *Config) ICloudDir(name string) string {
	r := c.Resolved()
	return filepath.Join(r.Storage.ICloudRoot, name)
}

// ProjectsDir returns the resolved projects directory.
// In icloud mode, this is under the iCloud root.
// In local mode, this is the paths.projects value directly.
func (c *Config) ProjectsDir() string {
	if c.IsICloud() {
		return c.ICloudDir("projects")
	}
	return ExpandPath(c.Paths.Projects)
}

// LifeDir returns the resolved life directory.
func (c *Config) LifeDir() string {
	if c.IsICloud() {
		return c.ICloudDir("life")
	}
	return ExpandPath(c.Paths.Life)
}

// WorkDir returns the resolved work directory.
func (c *Config) WorkDir() string {
	if c.IsICloud() {
		return c.ICloudDir("work")
	}
	return ExpandPath(c.Paths.Work)
}

// ArchiveDir returns the resolved archive directory.
func (c *Config) ArchiveDir() string {
	if c.IsICloud() {
		return c.ICloudDir("archive")
	}
	return ExpandPath(c.Paths.Archive)
}

// BackupDir returns the resolved backup directory.
// In icloud mode, this is under the iCloud root.
// In local mode, this is ~/.backups.
func (c *Config) BackupDir() string {
	if c.IsICloud() {
		return c.ICloudDir("backups")
	}
	return ExpandPath("~/.backups")
}

// CodeDir returns the resolved code root path.
func (c *Config) CodeDir() string {
	return ExpandPath(c.Paths.Code)
}

// isDir checks if a path is a directory (used for auto-detection).
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
