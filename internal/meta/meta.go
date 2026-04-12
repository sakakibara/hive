package meta

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const FileName = ".hive.json"

// ProjectMeta represents the metadata stored in each project's .hive.json file.
type ProjectMeta struct {
	Version   int               `json:"version"`
	Name      string            `json:"name"`
	Org       string            `json:"org"`
	CodeRel   string            `json:"code_rel,omitempty"`
	Repos     map[string]string `json:"repos,omitempty"`
	CreatedAt string            `json:"created_at"`
}

// New creates a new ProjectMeta with the current timestamp.
func New(name, org, codeRel string) *ProjectMeta {
	return &ProjectMeta{
		Version:   1,
		Name:      name,
		Org:       org,
		CodeRel:   codeRel,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// Write serializes the metadata to the given path as indented JSON.
func Write(path string, m *ProjectMeta) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project metadata: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// Read deserializes metadata from the given .hive.json path.
func Read(path string) (*ProjectMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m ProjectMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

// Validate checks that required fields are present and returns any issues.
func (m *ProjectMeta) Validate() []string {
	var issues []string
	if m.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}
	if m.Name == "" {
		issues = append(issues, "name is required")
	}
	if m.Org == "" {
		issues = append(issues, "org is required")
	}
	if m.CreatedAt == "" {
		issues = append(issues, "created_at is required")
	}
	return issues
}

// HasCode returns true if the project has an associated code directory.
func (m *ProjectMeta) HasCode() bool {
	return m.CodeRel != ""
}

// AddRepo records a repo URL by name. Initializes the map if needed.
func (m *ProjectMeta) AddRepo(name, url string) {
	if m.Repos == nil {
		m.Repos = make(map[string]string)
	}
	m.Repos[name] = url
}
