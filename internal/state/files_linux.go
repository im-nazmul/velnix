//go:build linux

package state

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"velnix/internal/domain"
)

const maxFile = 1024 * 1024

// dir holds an open directory descriptor. All leaf operations stay relative to it.
type dir struct{ f *os.File }

func openDir(path string, create bool) (*dir, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) == "/" {
		return nil, domain.E(domain.Invalid, "state directory must be an absolute non-root path")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/")
	for i, part := range parts {
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(e, unix.ENOENT) && create {
			e = unix.Mkdirat(fd, part, 0700)
			if e == nil {
				e = unix.Fsync(fd)
			}
			if e == nil || errors.Is(e, unix.EEXIST) {
				next, e = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
		}
		unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
		if i == len(parts)-1 {
			var s unix.Stat_t
			if e = unix.Fstat(fd, &s); e != nil {
				unix.Close(fd)
				return nil, e
			}
			if s.Uid != uint32(os.Geteuid()) || s.Mode&0777 != 0700 {
				unix.Close(fd)
				return nil, domain.E(domain.Denied, "%s must be owned by the current user with mode 0700", path)
			}
		}
	}
	return &dir{os.NewFile(uintptr(fd), path)}, nil
}
func (d *dir) close() { _ = d.f.Close() }
func (d *dir) open(name string, flags int) (*os.File, error) {
	fd, err := unix.Openat(int(d.f.Fd()), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	var s unix.Stat_t
	if err = unix.Fstat(fd, &s); err != nil {
		f.Close()
		return nil, err
	}
	if s.Mode&unix.S_IFMT != unix.S_IFREG || s.Uid != uint32(os.Geteuid()) || s.Mode&0777 != 0600 || s.Nlink != 1 {
		f.Close()
		return nil, domain.E(domain.Denied, "%s must be a private regular file owned by the current user without hard links", name)
	}
	return f, nil
}
func (d *dir) read(name string) ([]byte, error) {
	f, err := d.open(name, unix.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if len(b) > maxFile {
		return nil, domain.E(domain.Invalid, "%s exceeds size limit", name)
	}
	return b, err
}
func (d *dir) write(name string, b []byte) error {
	return d.publish(name, b, false)
}
func (d *dir) publish(name string, b []byte, createOnly bool) error {
	token := make([]byte, 12)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	temp := ".tmp-" + hex.EncodeToString(token)
	f, err := d.open(temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(int(d.f.Fd()), temp, 0)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Validate an existing destination before replacement, rejecting symlink/hardlink leaves.
	old, e := d.open(name, unix.O_RDONLY)
	if e == nil {
		old.Close()
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if createOnly {
		err = unix.Renameat2(int(d.f.Fd()), temp, int(d.f.Fd()), name, unix.RENAME_NOREPLACE)
	} else {
		err = unix.Renameat(int(d.f.Fd()), temp, int(d.f.Fd()), name)
	}
	if err != nil {
		return err
	}
	return d.f.Sync()
}
func readAt(path, name string) ([]byte, error) {
	d, err := openDir(path, false)
	if err != nil {
		return nil, err
	}
	defer d.close()
	return d.read(name)
}
func ioError(err error) error {
	if err == nil {
		return nil
	}
	var e *domain.Error
	if errors.As(err, &e) {
		return err
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		return domain.E(domain.Denied, "unsafe or inaccessible state path: %v", err)
	}
	if errors.Is(err, unix.EEXIST) {
		return domain.E(domain.Conflict, "state destination already exists; preview again")
	}
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) {
		return domain.E(domain.Unavailable, "filesystem/kernel does not support required durable state operation: %v", err)
	}
	return fmt.Errorf("state: %w", err)
}
