//go:build linux

package state

import (
	"context"
	"errors"
	"os"
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
func (s Store) ReadHTTP() ([]byte, error) {
	b, e := readAt(s.Paths.State, "http.json")
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
