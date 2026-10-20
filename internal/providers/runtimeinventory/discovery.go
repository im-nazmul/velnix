// Package runtimeinventory discovers external runtime candidates without running
// executables, reading project configuration or invoking runtime managers.
package runtimeinventory

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"velnix/internal/domain"
)

type Candidate struct {
	Path      string `json:"path"`
	Origin    string `json:"origin"`
	Authority string `json:"authority"`
	Probed    bool   `json:"probed"`
}

type Issue struct {
	Directory string `json:"directory"`
	Reason    string `json:"reason"`
}

type Inventory struct {
	Kind       string      `json:"kind"`
	Candidates []Candidate `json:"candidates"`
	Issues     []Issue     `json:"issues"`
}

var names = map[string]*regexp.Regexp{
	"node":   regexp.MustCompile(`^(node|nodejs)$`),
	"bun":    regexp.MustCompile(`^bun$`),
	"python": regexp.MustCompile(`^python(?:[0-9]+(?:\.[0-9]+)*)?$`),
}

// Discover reads only the explicitly supplied directories. Names are hints,
// never evidence of runtime identity, version, provenance or management rights.
func Discover(kind string, dirs []string) (Inventory, error) {
	out := Inventory{Kind: kind, Candidates: []Candidate{}, Issues: []Issue{}}
	pattern, ok := names[kind]
	if !ok {
		return out, domain.E(domain.Invalid, "runtime kind must be node, bun or python")
	}
	for _, dir := range dirs {
		if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
			return out, domain.E(domain.Invalid, "discovery directory must be an absolute clean path")
		}
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			reason := "unavailable"
			if os.IsPermission(err) {
				reason = "permission-denied"
			}
			out.Issues = append(out.Issues, Issue{Directory: dir, Reason: reason})
			continue
		}
		for _, entry := range entries {
			if !pattern.MatchString(entry.Name()) {
				continue
			}
			path, err := filepath.EvalSymlinks(filepath.Join(dir, entry.Name()))
			if err != nil || seen[path] {
				continue
			}
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
				continue
			}
			seen[path] = true
			out.Candidates = append(out.Candidates, Candidate{Path: path, Origin: "unknown", Authority: "external"})
		}
	}
	sort.Slice(out.Candidates, func(i, j int) bool { return out.Candidates[i].Path < out.Candidates[j].Path })
	return out, nil
}
