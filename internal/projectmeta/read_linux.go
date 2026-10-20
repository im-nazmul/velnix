//go:build linux

// Package projectmeta reads bounded literal metadata without following symlinks
// or executing configuration. It never scans project trees or opens devices.
package projectmeta

import (
	"golang.org/x/sys/unix"
	"io"
	"os"
	"strings"
	"velnix/internal/domain"
)

const MaxFile = 64 * 1024

func Read(root, name string) ([]byte, error) {
	if !domain.ValidRelativeMetadata(name) {
		return nil, domain.E(domain.Invalid, "invalid metadata path")
	}
	fd, e := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, err := unix.Openat(fd, part, flags, 0)
		unix.Close(fd)
		if err != nil {
			return nil, err
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > MaxFile {
		return nil, domain.E(domain.Invalid, "metadata %s must be a regular file of at most 64 KiB", name)
	}
	b, e := io.ReadAll(io.LimitReader(f, MaxFile+1))
	if len(b) > MaxFile {
		return nil, domain.E(domain.Invalid, "metadata exceeds limit")
	}
	return b, e
}
