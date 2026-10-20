//go:build linux

package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// HTTPFiles keeps HTTP evidence private and descriptor-relative. Callers use
// WithHTTP for writes so HTTP and project mutations share one lock.
type HTTPFiles struct{ d *dir }

func (f HTTPFiles) Read(name string) ([]byte, error) {
	b, e := f.d.read(name)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	return b, ioError(e)
}
func (f HTTPFiles) Write(name string, b []byte) error { return ioError(f.d.write(name, b)) }
func (s Store) ReadHTTP() ([]byte, error)             { return s.ReadWorkload("http.json") }
func (s Store) ReadWorkload(name string) ([]byte, error) {
	if strings.ContainsAny(name, "/\\") || name == "" {
		return nil, fmt.Errorf("invalid workload filename")
	}
	b, e := readAt(s.Paths.State, name)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	return b, ioError(e)
}
func (s Store) WithHTTP(ctx context.Context, fn func(HTTPFiles) error) error {
	d, e := openDir(s.Paths.State, false)
	if e != nil {
		return ioError(e)
	}
	defer d.close()
	f, e := d.open("mutation.lock", syscall.O_RDWR|syscall.O_CREAT)
	if e != nil {
		return ioError(e)
	}
	defer f.Close()
	for {
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			break
		}
		if !errors.Is(e, syscall.EWOULDBLOCK) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	if e = ctx.Err(); e != nil {
		return e
	}
	return fn(HTTPFiles{d})
}

// WithDirectory accesses a private child without following aliases. Creation is
// exclusive: interrupted generations are preserved, never silently reused.
func (f HTTPFiles) WithDirectory(name string, create bool, fn func(HTTPFiles) error) error {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("invalid private directory name")
	}
	if create {
		if e := syscall.Mkdirat(int(f.d.f.Fd()), name, 0700); e != nil {
			return e
		}
		if e := f.d.f.Sync(); e != nil {
			return e
		}
	}
	path := fmt.Sprintf("/proc/self/fd/%d/%s", f.d.f.Fd(), name)
	// openDir deliberately rejects symlink traversal, so open the child at its
	// pinned parent descriptor rather than traversing /proc.
	fd, e := syscall.Openat(int(f.d.f.Fd()), name, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	d := &dir{os.NewFile(uintptr(fd), path)}
	defer d.close()
	st, e := d.f.Stat()
	if e != nil {
		return e
	}
	s := st.Sys().(*syscall.Stat_t)
	if st.Mode().Perm() != 0700 || s.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("unsafe private directory")
	}
	if e = fn(HTTPFiles{d}); e != nil {
		return e
	}
	if create {
		return d.f.Sync()
	}
	return nil
}

// ProtectGenerated accepts only singly linked current-user regular files in a
// fresh private generation. mkcert writes public certificates with mode 0644.
func (f HTTPFiles) ProtectGenerated(name string) error {
	if strings.ContainsAny(name, "/\\") || name == "" {
		return fmt.Errorf("invalid generated filename")
	}
	fd, e := syscall.Openat(int(f.d.f.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	info, e := file.Stat()
	if e != nil {
		return e
	}
	st := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 || info.Size() > maxFile {
		return fmt.Errorf("unsafe generated file")
	}
	if e = file.Chmod(0600); e != nil {
		return e
	}
	return file.Sync()
}
