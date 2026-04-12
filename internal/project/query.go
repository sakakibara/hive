package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sakakibara/hive/internal/config"
	"github.com/sakakibara/hive/internal/meta"
)

// Scan finds all projects under the projects root by looking for .hive.json files.
func Scan(cfg *config.Config) ([]*Project, error) {
	resolved := cfg.Resolved()
	return scanDir(resolved.Paths.Projects, resolved.Paths.Code)
}

// ScanArchive finds all archived projects under Archive/projects.
func ScanArchive(cfg *config.Config) ([]*Project, error) {
	resolved := cfg.Resolved()
	archiveProjectsDir := filepath.Join(resolved.Paths.Archive, "projects")
	return scanDir(archiveProjectsDir, resolved.Paths.Code)
}

// FindByQuery returns all projects matching the given query using subpath matching.
// Matching is tiered: exact > prefix > substring > subsequence. Within each tier,
// smartcase applies (all-lowercase query is case-insensitive, mixed-case is exact).
// The highest-priority tier with results wins. Subsequence matches are sorted by
// span tightness.
func FindByQuery(cfg *config.Config, query string) ([]*Project, error) {
	all, err := Scan(cfg)
	if err != nil {
		return nil, err
	}
	return filterByQuery(all, query), nil
}

// FindArchivedByQuery returns archived projects matching the given query.
func FindArchivedByQuery(cfg *config.Config, query string) ([]*Project, error) {
	all, err := ScanArchive(cfg)
	if err != nil {
		return nil, err
	}
	return filterByQuery(all, query), nil
}

// scanDir walks a directory tree looking for .hive.json files and returns the projects found.
func scanDir(root, codeRoot string) ([]*Project, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", root, err)
	}

	if info, err := os.Stat(resolvedRoot); err != nil || !info.IsDir() {
		return nil, nil
	}

	var projects []*Project

	err = filepath.Walk(resolvedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() != meta.FileName {
			return nil
		}

		m, err := meta.Read(path)
		if err != nil {
			return nil
		}

		projRoot := filepath.Dir(path)

		p := &Project{
			Name:        m.Name,
			Org:         m.Org,
			ProjectRoot: projRoot,
			CodeRel:     m.CodeRel,
			Repos:       m.Repos,
			Meta:        m,
		}

		if m.HasCode() {
			p.CodeRoot = filepath.Join(codeRoot, m.CodeRel)
		}

		projects = append(projects, p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return projects, nil
}

// Subpaths returns all suffix paths for matching.
func (p *Project) Subpaths() []string {
	rel := filepath.Join(p.Org, p.Name)
	parts := strings.Split(rel, string(filepath.Separator))
	subs := make([]string, len(parts))
	for i := range parts {
		subs[i] = strings.Join(parts[i:], string(filepath.Separator))
	}
	return subs
}

// filterByQuery filters a list of projects by tiered smartcase subpath matching.
func filterByQuery(projects []*Project, query string) []*Project {
	caseSensitive := query != strings.ToLower(query)

	var exact, prefix, substring []*Project
	type subseqEntry struct {
		project *Project
		span    int
	}
	var subsequence []subseqEntry
	for _, p := range projects {
		best := matchTier(query, p.Subpaths(), caseSensitive)
		switch best {
		case tierExact:
			exact = append(exact, p)
		case tierPrefix:
			prefix = append(prefix, p)
		case tierSubstring:
			substring = append(substring, p)
		case tierSubsequence:
			span := bestSubsequenceSpan(query, p.Subpaths(), caseSensitive)
			subsequence = append(subsequence, subseqEntry{project: p, span: span})
		}
	}

	if len(exact) > 0 {
		return exact
	}
	if len(prefix) > 0 {
		return prefix
	}
	if len(substring) > 0 {
		return substring
	}
	if len(subsequence) > 0 {
		sort.Slice(subsequence, func(i, j int) bool {
			return subsequence[i].span < subsequence[j].span
		})
		result := make([]*Project, len(subsequence))
		for i, e := range subsequence {
			result[i] = e.project
		}
		return result
	}
	return nil
}

type matchTierLevel int

const (
	tierNone        matchTierLevel = iota
	tierSubsequence
	tierSubstring
	tierPrefix
	tierExact
)

// matchTier returns the best match tier for any of the candidate subpaths.
func matchTier(query string, subpaths []string, caseSensitive bool) matchTierLevel {
	best := tierNone
	for _, sub := range subpaths {
		q, s := query, sub
		if !caseSensitive {
			q = strings.ToLower(q)
			s = strings.ToLower(s)
		}

		if q == s {
			return tierExact
		}
		if strings.HasPrefix(s, q) && best < tierPrefix {
			best = tierPrefix
		} else if strings.Contains(s, q) && best < tierSubstring {
			best = tierSubstring
		} else if best < tierSubsequence && subsequenceSpan(q, s, true) >= 0 {
			best = tierSubsequence
		}
	}
	return best
}

// subsequenceSpan checks whether query is a subsequence of candidate and returns
// the span (last matched index - first matched index). Returns -1 if not a match.
// When caseSensitive is false, both strings are lowercased before comparison.
func subsequenceSpan(query, candidate string, caseSensitive bool) int {
	q, c := query, candidate
	if !caseSensitive {
		q = strings.ToLower(q)
		c = strings.ToLower(c)
	}

	qRunes := []rune(q)
	cRunes := []rune(c)

	if len(qRunes) == 0 {
		return -1
	}

	first := -1
	last := -1
	qi := 0
	for ci := 0; ci < len(cRunes) && qi < len(qRunes); ci++ {
		if cRunes[ci] == qRunes[qi] {
			if first == -1 {
				first = ci
			}
			last = ci
			qi++
		}
	}

	if qi < len(qRunes) {
		return -1
	}
	return last - first
}

// bestSubsequenceSpan returns the tightest (smallest) subsequence span across
// all subpaths. Returns -1 if no subpath matches.
func bestSubsequenceSpan(query string, subpaths []string, caseSensitive bool) int {
	best := -1
	for _, sub := range subpaths {
		span := subsequenceSpan(query, sub, caseSensitive)
		if span >= 0 && (best == -1 || span < best) {
			best = span
		}
	}
	return best
}
