package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~user/path", "~user/path"},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input)
		if got != tt.want {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadFrom_NonExistent(t *testing.T) {
	_, err := LoadFrom("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got: %v", err)
	}
}

func TestLoadFrom_ValidTOML_ICloud(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"

[storage]
mode = "icloud"
icloud_root = "~/Library/Mobile Documents/com~apple~CloudDocs/workspace"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Mode != ModeICloud {
		t.Errorf("expected icloud mode, got: %s", cfg.Storage.Mode)
	}
	if cfg.Storage.ICloudRoot == "" {
		t.Error("expected icloud_root to be set")
	}
	if cfg.Paths.Projects != "~/Projects" {
		t.Errorf("unexpected projects: %s", cfg.Paths.Projects)
	}
	if cfg.Paths.Code != "~/Code" {
		t.Errorf("unexpected code: %s", cfg.Paths.Code)
	}
}

func TestLoadFrom_ValidTOML_Local(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"

[storage]
mode = "local"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Mode != ModeLocal {
		t.Errorf("expected local mode, got: %s", cfg.Storage.Mode)
	}
}

func TestLoadFrom_MissingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected validation error for missing mode")
	}
}

func TestLoadFrom_InvalidMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"

[storage]
mode = "invalid"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected validation error for invalid mode")
	}
}

func TestLoadFrom_ICloudMissingRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[paths]
projects = "~/Projects"
life = "~/Life"
work = "~/Work"
archive = "~/Archive"
code = "~/Code"

[storage]
mode = "icloud"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected validation error for missing icloud_root")
	}
}

func TestResolved(t *testing.T) {
	cfg := &Config{
		Paths: PathsConfig{
			Projects: "~/Projects",
			Life:     "~/Life",
			Work:     "~/Work",
			Archive:  "~/Archive",
			Code:     "~/Code",
		},
		Storage: StorageConfig{
			Mode:       ModeICloud,
			ICloudRoot: "~/Library/Mobile Documents/com~apple~CloudDocs/workspace",
		},
	}

	resolved := cfg.Resolved()

	home, _ := os.UserHomeDir()
	if !strings.HasPrefix(resolved.Paths.Code, home) {
		t.Errorf("resolved code should start with home dir, got: %s", resolved.Paths.Code)
	}
	if strings.Contains(resolved.Paths.Code, "~") {
		t.Error("resolved path should not contain ~")
	}
	if !strings.HasPrefix(resolved.Storage.ICloudRoot, home) {
		t.Errorf("resolved icloud_root should start with home dir, got: %s", resolved.Storage.ICloudRoot)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nest", "config.toml")

	cfg := &Config{
		Paths: PathsConfig{
			Projects: "~/Projects",
			Life:     "~/Life",
			Work:     "~/Work",
			Archive:  "~/Archive",
			Code:     "~/Code",
		},
		Storage: StorageConfig{
			Mode:       ModeLocal,
		},
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Storage.Mode != ModeLocal {
		t.Errorf("expected local mode after reload, got: %s", loaded.Storage.Mode)
	}
	if loaded.Paths.Projects != "~/Projects" {
		t.Errorf("expected ~/Projects, got: %s", loaded.Paths.Projects)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid local",
			cfg: Config{
				Paths:   PathsConfig{Projects: "~/P", Life: "~/L", Work: "~/W", Archive: "~/A", Code: "~/C"},
				Storage: StorageConfig{Mode: ModeLocal},
			},
		},
		{
			name: "valid icloud",
			cfg: Config{
				Paths:   PathsConfig{Projects: "~/P", Life: "~/L", Work: "~/W", Archive: "~/A", Code: "~/C"},
				Storage: StorageConfig{Mode: ModeICloud, ICloudRoot: "~/icloud"},
			},
		},
		{
			name: "missing mode",
			cfg: Config{
				Paths: PathsConfig{Projects: "~/P", Life: "~/L", Work: "~/W", Archive: "~/A", Code: "~/C"},
			},
			wantErr: true,
		},
		{
			name: "missing projects",
			cfg: Config{
				Paths:   PathsConfig{Life: "~/L", Work: "~/W", Archive: "~/A", Code: "~/C"},
				Storage: StorageConfig{Mode: ModeLocal},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsICloud(t *testing.T) {
	icloud := &Config{Storage: StorageConfig{Mode: ModeICloud}}
	local := &Config{Storage: StorageConfig{Mode: ModeLocal}}

	if !icloud.IsICloud() {
		t.Error("expected IsICloud() = true for icloud mode")
	}
	if local.IsICloud() {
		t.Error("expected IsICloud() = false for local mode")
	}
}

func TestProjectsDir_ICloud(t *testing.T) {
	cfg := &Config{
		Paths:   PathsConfig{Projects: "/home/user/Projects"},
		Storage: StorageConfig{Mode: ModeICloud, ICloudRoot: "/home/user/icloud/workspace"},
	}

	got := cfg.ProjectsDir()
	want := "/home/user/icloud/workspace/projects"
	if got != want {
		t.Errorf("ProjectsDir() = %q, want %q", got, want)
	}
}

func TestProjectsDir_Local(t *testing.T) {
	cfg := &Config{
		Paths:   PathsConfig{Projects: "/home/user/Projects"},
		Storage: StorageConfig{Mode: ModeLocal},
	}

	got := cfg.ProjectsDir()
	want := "/home/user/Projects"
	if got != want {
		t.Errorf("ProjectsDir() = %q, want %q", got, want)
	}
}
