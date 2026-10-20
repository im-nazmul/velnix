package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var projectName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// Project owns a registry entry only; its source directory remains external.
type Project struct {
	Name           string       `json:"name"`
	Root           string       `json:"root"`
	DocumentRoot   string       `json:"document_root"`
	Domain         string       `json:"domain"`
	Device         uint64       `json:"device"`
	Inode          uint64       `json:"inode"`
	DocumentDevice uint64       `json:"document_device"`
	DocumentInode  uint64       `json:"document_inode"`
	PHPCLI         PHPSelection `json:"php_cli,omitzero"`
}

func ValidProjectName(name string) bool { return projectName.MatchString(name) }

// Validate checks stored metadata without accessing project files or executing code.
func (p Project) Validate() error {
	if p.PHPCLI != (PHPSelection{}) {
		if err := p.PHPCLI.Validate(); err != nil {
			return err
		}
	}
	if !ValidProjectName(p.Name) || p.Domain != p.Name+".test" {
		return E(Invalid, "invalid project name or domain")
	}
	if !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root || !filepath.IsAbs(p.DocumentRoot) || filepath.Clean(p.DocumentRoot) != p.DocumentRoot {
		return E(Invalid, "project paths must be absolute and canonical")
	}
	rel, err := filepath.Rel(p.Root, p.DocumentRoot)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return E(Invalid, "document root must be inside the project root")
	}
	if p.Inode == 0 || p.DocumentInode == 0 {
		return E(Invalid, "project directory identity is missing")
	}
	return nil
}

// ResolveProject canonicalizes paths and binds approval to observed directory identities.
// It reads directory metadata only; no framework files or hooks are loaded.
func ResolveProject(name, root, documentRoot string) (Project, error) {
	if !ValidProjectName(name) {
		return Project{}, E(Invalid, "name must be a lowercase DNS label of 1–63 letters, digits or hyphens")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Project{}, E(Invalid, "resolve project path: %v", err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Project{}, E(Unavailable, "project directory unavailable: %v", err)
	}
	if !filepath.IsAbs(documentRoot) {
		documentRoot = filepath.Join(canonical, documentRoot)
	}
	documentRoot, err = filepath.EvalSymlinks(documentRoot)
	if err != nil {
		return Project{}, E(Unavailable, "document root unavailable: %v", err)
	}
	p := Project{Name: name, Root: canonical, DocumentRoot: documentRoot, Domain: name + ".test"}
	identity := func(path string) (uint64, uint64, error) {
		info, e := os.Stat(path)
		if e != nil {
			return 0, 0, E(Unavailable, "directory unavailable: %v", e)
		}
		if !info.IsDir() {
			return 0, 0, E(Invalid, "project and document roots must be directories")
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return 0, 0, E(Unavailable, "directory identity unavailable")
		}
		return uint64(st.Dev), st.Ino, nil
	}
	p.Device, p.Inode, err = identity(p.Root)
	if err != nil {
		return Project{}, err
	}
	p.DocumentDevice, p.DocumentInode, err = identity(p.DocumentRoot)
	if err != nil {
		return Project{}, err
	}
	return p, p.Validate()
}
