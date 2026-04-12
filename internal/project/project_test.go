package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/meta"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	tmp := t.TempDir()
	return &config.Config{
		Paths: config.PathsConfig{
			Projects: filepath.Join(tmp, "Projects"),
			Life:     filepath.Join(tmp, "Life"),
			Work:     filepath.Join(tmp, "Work"),
			Archive:  filepath.Join(tmp, "Archive"),
			Code:     filepath.Join(tmp, "Code"),
		},
		Storage: config.StorageConfig{
			Mode: config.ModeLocal,
		},
	}
}

func TestResolveNew_WithoutCode(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()

	p := ResolveNew(cfg, "mycompany", "hr-strategy")

	wantProjectRoot := filepath.Join(resolved.Paths.Projects, "mycompany", "hr-strategy")
	if p.ProjectRoot != wantProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", p.ProjectRoot, wantProjectRoot)
	}
	if p.CodeRoot != "" {
		t.Errorf("CodeRoot should be empty, got %q", p.CodeRoot)
	}
	if p.HasCode() {
		t.Error("expected HasCode() = false")
	}
}

func TestResolveNewWithCode(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()

	p := ResolveNewWithCode(cfg, "acme", "order-form")

	wantProjectRoot := filepath.Join(resolved.Paths.Projects, "acme", "order-form")
	if p.ProjectRoot != wantProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", p.ProjectRoot, wantProjectRoot)
	}
	wantCodeRoot := filepath.Join(resolved.Paths.Code, "acme", "order-form")
	if p.CodeRoot != wantCodeRoot {
		t.Errorf("CodeRoot = %q, want %q", p.CodeRoot, wantCodeRoot)
	}
	if p.CodeRel != "acme/order-form" {
		t.Errorf("CodeRel = %q, want %q", p.CodeRel, "acme/order-form")
	}
	if !p.HasCode() {
		t.Error("expected HasCode() = true")
	}
}

func TestCreateWithoutCode(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()
	os.MkdirAll(resolved.Paths.Projects, 0755)

	p := ResolveNew(cfg, "mycompany", "hr-strategy")
	if err := Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := os.Stat(p.ProjectRoot); err != nil {
		t.Fatalf("project root missing: %v", err)
	}

	// No code symlink.
	codeLinkPath := filepath.Join(p.ProjectRoot, "code")
	if _, err := os.Lstat(codeLinkPath); !os.IsNotExist(err) {
		t.Error("code symlink should not exist")
	}

	// Subdirs should exist.
	for _, sub := range Subdirs {
		if _, err := os.Stat(filepath.Join(p.ProjectRoot, sub)); err != nil {
			t.Errorf("subdir %q missing", sub)
		}
	}

	// Metadata.
	m, err := meta.Read(filepath.Join(p.ProjectRoot, meta.FileName))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if m.HasCode() {
		t.Error("metadata should have no code_rel")
	}
}

func TestCreateWithCode(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()
	os.MkdirAll(resolved.Paths.Projects, 0755)
	os.MkdirAll(resolved.Paths.Code, 0755)

	p := ResolveNewWithCode(cfg, "personal", "myproj")
	if err := Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Code dir should exist (as a container).
	if _, err := os.Stat(p.CodeRoot); err != nil {
		t.Fatalf("code root missing: %v", err)
	}

	// Code symlink should point to container.
	codeLinkPath := filepath.Join(p.ProjectRoot, "code")
	target, err := os.Readlink(codeLinkPath)
	if err != nil {
		t.Fatalf("code symlink: %v", err)
	}
	if target != p.CodeRoot {
		t.Errorf("code symlink target = %q, want %q", target, p.CodeRoot)
	}
}

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme/order-form-api.git", "order-form-api"},
		{"https://github.com/acme/order-form-api", "order-form-api"},
		{"git@github.com:acme/frontend.git", "frontend"},
		{"git@github.com:acme/frontend", "frontend"},
		{"https://gitlab.com/group/subgroup/repo.git", "repo"},
	}
	for _, tt := range tests {
		got := RepoNameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("RepoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestScanAndFind(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()
	os.MkdirAll(resolved.Paths.Projects, 0755)
	os.MkdirAll(resolved.Paths.Code, 0755)

	Create(ResolveNew(cfg, "mycompany", "strategy"))
	Create(ResolveNewWithCode(cfg, "acme", "order-form"))

	projects, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	tests := []struct {
		query string
		want  int
	}{
		{"strategy", 1},
		{"mycompany/strategy", 1},
		{"order-form", 1},
		{"acme/order-form", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		matches, err := FindByQuery(cfg, tt.query)
		if err != nil {
			t.Fatalf("FindByQuery(%q): %v", tt.query, err)
		}
		if len(matches) != tt.want {
			t.Errorf("FindByQuery(%q) = %d matches, want %d", tt.query, len(matches), tt.want)
		}
	}
}

func TestScan_ThroughSymlink(t *testing.T) {
	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "real_projects")
	linkDir := filepath.Join(tmp, "Projects")

	os.MkdirAll(realDir, 0755)
	os.Symlink(realDir, linkDir)

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Projects: linkDir,
			Life:     filepath.Join(tmp, "Life"),
			Work:     filepath.Join(tmp, "Work"),
			Archive:  filepath.Join(tmp, "Archive"),
			Code:     filepath.Join(tmp, "Code"),
		},
		Storage: config.StorageConfig{
			Mode: config.ModeLocal,
		},
	}

	os.MkdirAll(filepath.Join(tmp, "Code"), 0755)

	p := ResolveNewWithCode(cfg, "personal", "symtest")
	if err := Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	projects, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "symtest" {
		t.Errorf("scanned name = %q", projects[0].Name)
	}
}

func TestCreate_Conflict(t *testing.T) {
	cfg := testConfig(t)
	resolved := cfg.Resolved()
	os.MkdirAll(resolved.Paths.Projects, 0755)

	p := ResolveNew(cfg, "personal", "conflict")
	os.MkdirAll(p.ProjectRoot, 0755)

	err := Create(p)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestSubpaths(t *testing.T) {
	p := &Project{Org: "acme", Name: "order-form"}
	subs := p.Subpaths()
	want := []string{"acme/order-form", "order-form"}
	if len(subs) != len(want) {
		t.Fatalf("Subpaths() = %v, want %v", subs, want)
	}
	for i, s := range subs {
		if s != want[i] {
			t.Errorf("Subpaths()[%d] = %q, want %q", i, s, want[i])
		}
	}
}

func TestMatchTier(t *testing.T) {
	tests := []struct {
		query         string
		subpaths      []string
		caseSensitive bool
		want          matchTierLevel
	}{
		// Exact matches.
		{"foo", []string{"foo"}, false, tierExact},
		{"foo", []string{"Foo"}, false, tierExact},  // case-insensitive
		{"Foo", []string{"Foo"}, true, tierExact},    // case-sensitive exact
		{"Foo", []string{"foo"}, true, tierNone},     // case-sensitive mismatch

		// Prefix matches.
		{"hy", []string{"hypericum"}, false, tierPrefix},
		{"HY", []string{"hypericum"}, false, tierPrefix},
		{"Hy", []string{"Hypericum"}, true, tierPrefix},
		{"Hy", []string{"hypericum"}, true, tierNone},

		// Substring matches.
		{"per", []string{"hypericum"}, false, tierSubstring},
		{"form", []string{"order-form"}, false, tierSubstring},

		// Subsequence matches.
		{"wbs", []string{"website"}, false, tierSubsequence},
		{"odf", []string{"order-form"}, false, tierSubsequence},
		{"WBS", []string{"website"}, false, tierSubsequence}, // case-insensitive
		{"Wbs", []string{"Website"}, true, tierSubsequence},  // case-sensitive
		{"Wbs", []string{"website"}, true, tierNone},          // case-sensitive mismatch

		// Subsequence does NOT match when chars are out of order.
		{"sbw", []string{"website"}, false, tierNone},

		// No match.
		{"xyz", []string{"hypericum"}, false, tierNone},

		// Best tier wins across subpaths.
		{"foo", []string{"acme/foo", "foo"}, false, tierExact},
		{"hy", []string{"personal/hypericum", "hypericum"}, false, tierPrefix},
	}
	for _, tt := range tests {
		got := matchTier(tt.query, tt.subpaths, tt.caseSensitive)
		if got != tt.want {
			t.Errorf("matchTier(%q, %v, %v) = %v, want %v", tt.query, tt.subpaths, tt.caseSensitive, got, tt.want)
		}
	}
}

func TestSubsequenceSpan(t *testing.T) {
	tests := []struct {
		query, candidate string
		caseSensitive    bool
		want             int // -1 means no match
	}{
		{"wbs", "website", false, 3},   // w(0)b(2)s(3) → span=3
		{"wbt", "website", false, 5},   // w(0)b(2)t(5) → span=5
		{"sbw", "website", false, -1},  // no match
		{"ws", "website", false, 3},    // w(0)s(3) → span=3
		{"ws", "web-system", false, 4}, // w(0)s(4) → span=4
	}
	for _, tt := range tests {
		got := subsequenceSpan(tt.query, tt.candidate, tt.caseSensitive)
		if got != tt.want {
			t.Errorf("subsequenceSpan(%q, %q, %v) = %d, want %d", tt.query, tt.candidate, tt.caseSensitive, got, tt.want)
		}
	}
}
