//go:build linux

// Package phpcli handles existing native PHP CLI binaries, never shell shims.
package phpcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"

	"velnix/internal/domain"
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

// Snapshot reads metadata and bytes only. The opened descriptor is also used by
// Probe/Open so a path replacement cannot redirect the subsequent execution.
func snapshot(path, version string) (domain.PHPSelection, *os.File, error) {
	var s domain.PHPSelection
	if !filepath.IsAbs(path) || !domain.ValidPHPVersion(version) {
		return s, nil, domain.E(domain.Invalid, "PHP requires an absolute executable and exact version (for example 8.3.12)")
	}
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return s, nil, domain.E(domain.Unavailable, "PHP executable unavailable: %v", err)
	}
	fd, err := unix.Open(p, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return s, nil, domain.E(domain.Unavailable, "PHP executable unavailable: %v", err)
	}
	f := os.NewFile(uintptr(fd), p)
	fail := func(err error) (domain.PHPSelection, *os.File, error) { f.Close(); return s, nil, err }
	info, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || info.Size() > 128<<20 {
		return fail(domain.E(domain.Denied, "PHP must be a regular executable, at most 128 MiB, without group/world write or set-ID bits"))
	}
	var magic [4]byte
	if _, err = io.ReadFull(f, magic[:]); err != nil || magic != [4]byte{0x7f, 'E', 'L', 'F'} {
		return fail(domain.E(domain.Denied, "PHP selection requires a native ELF binary; scripts and manager shims are unsupported"))
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (128<<20)+1))
	if err != nil {
		return fail(err)
	}
	if n != info.Size() {
		return fail(domain.E(domain.Conflict, "PHP executable changed while reading"))
	}
	st := info.Sys().(*syscall.Stat_t)
	s = domain.PHPSelection{Path: p, Version: version, SHA256: hex.EncodeToString(h.Sum(nil)), Device: uint64(st.Dev), Inode: st.Ino}
	return s, f, nil
}

func Snapshot(path, version string) (domain.PHPSelection, error) {
	s, f, err := snapshot(path, version)
	if f != nil {
		f.Close()
	}
	return s, err
}

func Open(s domain.PHPSelection) (*os.File, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	now, f, err := snapshot(s.Path, s.Version)
	if err != nil {
		return nil, err
	}
	if now != s {
		f.Close()
		return nil, domain.E(domain.Conflict, "selected PHP executable changed; review a new selection")
	}
	return f, nil
}

func Command(f *os.File, s domain.PHPSelection, args ...string) *exec.Cmd {
	c := exec.Command("/proc/self/fd/3", args...)
	c.Args[0] = s.Path
	c.ExtraFiles = []*os.File{f}
	c.WaitDelay = time.Second
	return c
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
