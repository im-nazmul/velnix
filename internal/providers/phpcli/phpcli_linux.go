//go:build linux

// Package phpcli handles existing native PHP CLI binaries, never shell shims.
package phpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
)

type Candidate struct {
	Path      string `json:"path"`
	Origin    string `json:"origin"`
	Authority string `json:"authority"`
	Probed    bool   `json:"probed"`
}

var candidateName = regexp.MustCompile(`^php(?:[0-9]+(?:\.[0-9]+)*)?$`)

// Discover deliberately ignores PATH, profiles, repository files and managers.
func Discover() []Candidate {
	return DiscoverIn([]string{"/usr/bin", "/usr/local/bin"})
}

func DiscoverIn(dirs []string) []Candidate {
	out := []Candidate{}
	seen := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !candidateName.MatchString(name) {
				continue
			}
			p, err := filepath.EvalSymlinks(filepath.Join(dir, name))
			if err != nil || seen[p] {
				continue
			}
			info, err := os.Stat(p)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
				continue
			}
			seen[p] = true
			out = append(out, Candidate{Path: p, Origin: "unknown", Authority: "external"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Snapshot reads executable identity without running it.
func Snapshot(path, version string) (domain.PHPSelection, error) {
	return nativeexec.Snapshot(path, version)
}

func Open(s domain.PHPSelection) (*os.File, error) { return nativeexec.Open(s) }

func Command(f *os.File, s domain.PHPSelection, args ...string) *exec.Cmd {
	return nativeexec.Command(f, s, args...)
}

type bounded struct {
	bytes.Buffer
	overflow bool
}

func (b *bounded) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 4096 - b.Len()
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

// Probe uses no php.ini, extensions, project cwd, inherited loader variables or
// stdin. Approval is enforced by the application/transaction caller.
func Probe(ctx context.Context, s domain.PHPSelection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := Open(s)
	if err != nil {
		return err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	c := Command(f, s, "-n", "-r", `echo json_encode(["version"=>PHP_VERSION,"sapi"=>PHP_SAPI]);`)
	c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	c.Dir = "/"
	var out, stderr bounded
	c.Stdout, c.Stderr = &out, &stderr
	if err = c.Start(); err != nil {
		return domain.E(domain.Unavailable, "cannot start PHP probe: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = c.Process.Kill()
		<-done
		return ctx.Err()
	}
	if err != nil || out.overflow || stderr.Len() != 0 {
		return domain.E(domain.Unavailable, "PHP probe failed or produced unexpected diagnostics")
	}
	var result struct {
		Version string `json:"version"`
		SAPI    string `json:"sapi"`
	}
	if err = json.Unmarshal(out.Bytes(), &result); err != nil || result.Version != s.Version || result.SAPI != "cli" {
		return domain.E(domain.Unavailable, "PHP CLI does not match required exact version %s", s.Version)
	}
	// Detect in-place modification during the probe, too.
	check, err := Open(s)
	if err != nil {
		return err
	}
	check.Close()
	return nil
}
