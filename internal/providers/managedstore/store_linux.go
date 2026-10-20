//go:build linux

// Package managedstore resolves a bounded, explicit existing installation layout.
// It never invokes runtime managers, reads their config or infers ownership.
package managedstore

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
)

const Layout = "existing-store-v1"

var release = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type Request struct{ Provider, Root, Kind, Version string }
type Resolution struct {
	Provider    string                     `json:"provider"`
	Layout      string                     `json:"layout"`
	Root        string                     `json:"root"`
	Entry       string                     `json:"entry"`
	Executable  domain.ExecutableSelection `json:"executable"`
	Probed      bool                       `json:"probed"`
	Authority   string                     `json:"authority"`
	Origin      string                     `json:"origin"`
	Limitations string                     `json:"limitations"`
}

func Resolve(req Request) (Resolution, error) {
	var result Resolution
	if !filepath.IsAbs(req.Root) || filepath.Clean(req.Root) != req.Root || req.Root == "/" || !release.MatchString(req.Version) {
		return result, domain.E(domain.Invalid, "provider requires a non-root absolute clean storage directory and exact numeric release version")
	}
	if req.Provider != "mise" && req.Provider != "uv" {
		return result, domain.E(domain.Invalid, "provider must be mise or uv")
	}
	if req.Provider == "mise" && (req.Kind != "node" && req.Kind != "bun") || req.Provider == "uv" && req.Kind != "python" {
		return result, domain.E(domain.Invalid, "mise supports Node/Bun; uv supports Python")
	}
	root, e := filepath.EvalSymlinks(req.Root)
	if e != nil {
		return result, domain.E(domain.Unavailable, "provider storage unavailable: %v", e)
	}
	if root == "/" {
		return result, domain.E(domain.Invalid, "canonical provider storage root cannot be /")
	}
	info, e := os.Stat(root)
	if e != nil || !info.IsDir() {
		return result, domain.E(domain.Unavailable, "provider storage must be a directory")
	}
	entry := ""
	if req.Provider == "mise" {
		// Exact version links may point to the identity store under the explicit
		// data root. External MISE_INSTALL_STORE_DIR locations require direct paths.
		entry = filepath.Join("installs", req.Kind, req.Version, "bin", req.Kind)
	} else {
		arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
		if arch == "" {
			return result, domain.E(domain.Unavailable, "uv layout supports Linux amd64/arm64 GNU installations only")
		}
		expected := "cpython-" + req.Version + "-linux-" + arch + "-gnu"
		// Read at most 1025 names: unrelated stores cannot force an unbounded scan.
		f, e := os.Open(root)
		if e != nil {
			return result, e
		}
		names, e := f.Readdirnames(1025)
		f.Close()
		if e != nil && e != io.EOF {
			return result, e
		}
		if len(names) > 1024 {
			return result, domain.E(domain.Unavailable, "uv storage exceeds the 1024-entry inspection bound; use an exact native executable")
		}
		found := false
		for _, name := range names {
			if name == expected {
				found = true
			}
		}
		if !found {
			return result, domain.E(domain.Unavailable, "exact uv CPython installation %s is unavailable; nothing will be downloaded", expected)
		}
		parts := strings.Split(req.Version, ".")
		entry = filepath.Join(expected, "bin", "python"+parts[0]+"."+parts[1])
	}
	path := filepath.Join(root, entry)
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil {
		return result, domain.E(domain.Unavailable, "exact provider installation unavailable: %v; nothing will be installed", e)
	}
	relative, e := filepath.Rel(root, canonical)
	if e != nil || !domain.ValidRelativeMetadata(relative) {
		return result, domain.E(domain.Denied, "provider executable escapes the explicit storage root; review a direct --executable selection instead")
	}
	selected, e := nativeexec.Snapshot(canonical, req.Version)
	if e != nil {
		return result, e
	}
	result = Resolution{Provider: req.Provider, Layout: Layout, Root: root, Entry: entry, Executable: selected, Authority: "external", Origin: "unknown", Limitations: "Read-only storage-layout resolution, not a manager command or proof of installer provenance. Filename version is unverified until the approved exact-version probe. No downloads, hooks, activation, config reads, adoption or ownership grants. Unsupported layouts require a direct executable."}
	return result, nil
}
