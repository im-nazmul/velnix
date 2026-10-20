package domain

import (
	"encoding/hex"
	"path/filepath"
	"strings"
)

// RuntimeSelection owns metadata only; the executable and venv remain external.
type RuntimeSelection struct {
	Executable ExecutableSelection `json:"executable"`
	Venv       string              `json:"venv,omitempty"`
	VenvConfig string              `json:"venv_config_sha256,omitempty"`
	VenvDevice uint64              `json:"venv_device,omitempty"`
	VenvInode  uint64              `json:"venv_inode,omitempty"`
	Launcher   string              `json:"launcher,omitempty"`
}

func MultiRuntime(kind string) bool { return kind == "node" || kind == "bun" || kind == "python" }
func (s RuntimeSelection) Validate(kind string) error {
	if !MultiRuntime(kind) {
		return E(Invalid, "unsupported runtime kind")
	}
	if e := s.Executable.Validate(); e != nil {
		return e
	}
	if s.Venv == "" {
		if s.VenvConfig != "" || s.VenvDevice != 0 || s.VenvInode != 0 || s.Launcher != "" {
			return E(Invalid, "unexpected virtualenv metadata")
		}
		return nil
	}
	h, e := hex.DecodeString(s.VenvConfig)
	if kind != "python" || !filepath.IsAbs(s.Venv) || filepath.Clean(s.Venv) != s.Venv || s.VenvInode == 0 || e != nil || len(h) != 32 || s.Launcher != filepath.Join(s.Venv, "bin", "python") {
		return E(Invalid, "invalid Python virtual environment selection")
	}
	return nil
}
func (p Project) Runtime(kind string) RuntimeSelection {
	switch kind {
	case "node":
		return p.Node
	case "bun":
		return p.Bun
	case "python":
		return p.Python
	}
	return RuntimeSelection{}
}
func (p *Project) SetRuntime(kind string, s RuntimeSelection) {
	switch kind {
	case "node":
		p.Node = s
	case "bun":
		p.Bun = s
	case "python":
		p.Python = s
	}
}

// WithSettingsFrom copies metadata unrelated to source directory identity.
func (p Project) WithSettingsFrom(other Project) Project {
	p.PHPCLI = other.PHPCLI
	p.Node = other.Node
	p.Bun = other.Bun
	p.Python = other.Python
	p.ProxyPort = other.ProxyPort
	return p
}
func ValidProxyPort(port int) bool { return port == 0 || port >= 1024 && port <= 65535 }
func ValidRelativeMetadata(name string) bool {
	return name != "" && !filepath.IsAbs(name) && filepath.Clean(name) == name && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}
