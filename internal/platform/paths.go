package platform

import (
	"os"
	"path/filepath"

	"velnix/internal/domain"
)

type Paths struct {
	Config string `json:"config"`
	Data   string `json:"data"`
	State  string `json:"state"`
	Cache  string `json:"cache"`
}

// ResolvePaths never creates directories and ignores relative XDG values per XDG.
func ResolvePaths(getenv func(string) string) (Paths, error) {
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return Paths{}, domain.E(domain.Invalid, "HOME must be an absolute path")
	}
	base := func(key, fallback string) string {
		v := getenv(key)
		if !filepath.IsAbs(v) {
			v = filepath.Join(home, fallback)
		}
		return filepath.Join(v, "velnix")
	}
	return Paths{base("XDG_CONFIG_HOME", ".config"), base("XDG_DATA_HOME", ".local/share"), base("XDG_STATE_HOME", ".local/state"), base("XDG_CACHE_HOME", ".cache")}, nil
}
func UserPaths() (Paths, error) { return ResolvePaths(os.Getenv) }
