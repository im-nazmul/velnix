package domain

import (
	"encoding/hex"
	"path/filepath"
	"regexp"
)

// ExecutableSelection identifies one exact external native CLI installation.
// Identity does not grant installation, removal or persistent selection authority.
type ExecutableSelection struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
}

// PHPSelection preserves the existing PHP registry representation.
type PHPSelection = ExecutableSelection

var exactRuntimeVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[A-Za-z+-][A-Za-z0-9.+-]*)?$`)

func ValidRuntimeVersion(v string) bool { return len(v) <= 80 && exactRuntimeVersion.MatchString(v) }

func ValidPHPVersion(v string) bool { return ValidRuntimeVersion(v) }

func (s ExecutableSelection) Validate() error {
	b, err := hex.DecodeString(s.SHA256)
	if !filepath.IsAbs(s.Path) || filepath.Clean(s.Path) != s.Path || !ValidRuntimeVersion(s.Version) || err != nil || len(b) != 32 || s.Inode == 0 {
		return E(Invalid, "invalid exact native CLI selection")
	}
	return nil
}
