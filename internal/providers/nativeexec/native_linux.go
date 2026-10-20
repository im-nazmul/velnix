//go:build linux

// Package nativeexec pins existing native executable identities for approved probes.
package nativeexec

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"velnix/internal/domain"
)

// Snapshot reads metadata and bytes only. The opened descriptor is also used by
// Probe/Open so a path replacement cannot redirect the subsequent execution.
func snapshot(path, version string) (domain.ExecutableSelection, *os.File, error) {
	var s domain.ExecutableSelection
	if !filepath.IsAbs(path) || !domain.ValidRuntimeVersion(version) {
		return s, nil, domain.E(domain.Invalid, "runtime requires an absolute executable and exact version (for example 8.3.12)")
	}
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return s, nil, domain.E(domain.Unavailable, "runtime executable unavailable: %v", err)
	}
	fd, err := unix.Open(p, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return s, nil, domain.E(domain.Unavailable, "runtime executable unavailable: %v", err)
	}
	f := os.NewFile(uintptr(fd), p)
	fail := func(err error) (domain.ExecutableSelection, *os.File, error) { f.Close(); return s, nil, err }
	info, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || info.Size() > 128<<20 {
		return fail(domain.E(domain.Denied, "runtime must be a regular executable, at most 128 MiB, without group/world write or set-ID bits"))
	}
	var magic [4]byte
	if _, err = io.ReadFull(f, magic[:]); err != nil || magic != [4]byte{0x7f, 'E', 'L', 'F'} {
		return fail(domain.E(domain.Denied, "runtime selection requires a native ELF binary; scripts and manager shims are unsupported"))
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
		return fail(domain.E(domain.Conflict, "runtime executable changed while reading"))
	}
	st := info.Sys().(*syscall.Stat_t)
	s = domain.ExecutableSelection{Path: p, Version: version, SHA256: hex.EncodeToString(h.Sum(nil)), Device: uint64(st.Dev), Inode: st.Ino}
	return s, f, nil
}

func Snapshot(path, version string) (domain.ExecutableSelection, error) {
	s, f, err := snapshot(path, version)
	if f != nil {
		f.Close()
	}
	return s, err
}

func Open(s domain.ExecutableSelection) (*os.File, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	now, f, err := snapshot(s.Path, s.Version)
	if err != nil {
		return nil, err
	}
	if now != s {
		f.Close()
		return nil, domain.E(domain.Conflict, "selected runtime executable changed; review a new selection")
	}
	return f, nil
}

func Command(f *os.File, s domain.ExecutableSelection, args ...string) *exec.Cmd {
	c := exec.Command("/proc/self/fd/3", args...)
	c.Args[0] = s.Path
	c.ExtraFiles = []*os.File{f}
	c.WaitDelay = time.Second
	return c
}
