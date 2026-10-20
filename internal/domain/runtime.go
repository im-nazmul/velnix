package domain

import (
	"encoding/hex"
	"path/filepath"
	"regexp"
)

// PHPSelection records permission to run one exact external CLI installation.
// It grants no authority to install, replace or remove that installation.
type PHPSelection struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
}

var phpVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[A-Za-z+-][A-Za-z0-9.+-]*)?$`)

func ValidPHPVersion(v string) bool { return len(v) <= 80 && phpVersion.MatchString(v) }

func (s PHPSelection) Validate() error {
	b, err := hex.DecodeString(s.SHA256)
	if !filepath.IsAbs(s.Path) || filepath.Clean(s.Path) != s.Path || !ValidPHPVersion(s.Version) || err != nil || len(b) != 32 || s.Inode == 0 {
		return E(Invalid, "invalid exact PHP CLI selection")
	}
	return nil
}
