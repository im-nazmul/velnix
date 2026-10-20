package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/pelletier/go-toml/v2"

	"velnix/internal/domain"
)

const MaxBytes = 64 * 1024

type Config struct {
	Schema      int         `toml:"schema" json:"schema"`
	Diagnostics Diagnostics `toml:"diagnostics" json:"diagnostics"`
}
type Diagnostics struct {
	TimeoutMS int `toml:"timeout_ms" json:"timeout_ms"`
}

func Default() Config { return Config{Schema: 1, Diagnostics: Diagnostics{TimeoutMS: 2000}} }

type Result struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Config Config `json:"config"`
}

func Load(path string, required bool) (Result, error) {
	result := Result{Path: path, Config: Default()}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if errors.Is(err, os.ErrNotExist) && !required {
		return result, nil
	}
	if err != nil {
		return result, domain.E(domain.Invalid, "cannot read config: %v", err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, domain.E(domain.Invalid, "config must be a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return result, err
	}
	if len(b) > MaxBytes {
		return result, domain.E(domain.Invalid, "config exceeds %d bytes", MaxBytes)
	}
	if err = toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(&result.Config); err != nil {
		// Parser errors may include user input; do not echo config contents into diagnostics.
		return result, domain.E(domain.Invalid, "invalid TOML or unknown config key in %s", filepath.Base(path))
	}
	if result.Config.Schema != 1 {
		return result, domain.E(domain.Invalid, "unsupported config schema %d (supported: 1)", result.Config.Schema)
	}
	if result.Config.Diagnostics.TimeoutMS < 100 || result.Config.Diagnostics.TimeoutMS > 5000 {
		return result, domain.E(domain.Invalid, "diagnostics.timeout_ms must be between 100 and 5000")
	}
	result.Exists = true
	return result, nil
}
