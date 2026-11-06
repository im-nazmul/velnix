//go:build linux

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
	"velnix/internal/domain"
)

type UIRuntime struct {
	Base   string `json:"base"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func DashboardRuntime() (UIRuntime, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = filepath.Join(os.TempDir(), fmt.Sprintf("velnix-ui-%d", os.Geteuid()))
	}
	out := UIRuntime{Base: base}
	if !filepath.IsAbs(base) {
		return out, domain.E(domain.Invalid, "UI runtime directory must be absolute")
	}
	d, e := openDir(base, false)
	if errors.Is(e, os.ErrNotExist) {
		if os.Getenv("XDG_RUNTIME_DIR") != "" {
			return out, domain.E(domain.Unavailable, "configured XDG_RUNTIME_DIR does not exist")
		}
		return out, nil
	}
	if e != nil {
		return out, e
	}
	defer d.close()
	info, e := d.f.Stat()
	if e != nil {
		return out, e
	}
	s := info.Sys().(*syscall.Stat_t)
	out.Device = uint64(s.Dev)
	out.Inode = s.Ino
	return out, nil
}

// UIBootstrap retains descriptors to only the directory/file created for this
// launch. It never adopts or recursively deletes abandoned runtime entries.
type UIBootstrap struct {
	mu               sync.Mutex
	parent, session  *dir
	name             string
	expected         string
	Path             string
	consumed, closed bool
}

func CreateUIBootstrap(base, name, secret string) (*UIBootstrap, error) {
	if len(name) != 35 || name[:3] != "ui-" {
		return nil, domain.E(domain.Invalid, "invalid UI session name")
	}
	for _, c := range name[3:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, domain.E(domain.Invalid, "invalid UI session name")
		}
	}
	if len(secret) != 64 {
		return nil, domain.E(domain.Invalid, "invalid UI bootstrap secret")
	}
	parent, e := openDir(base, true)
	if e != nil {
		return nil, e
	}
	if e = unix.Mkdirat(int(parent.f.Fd()), name, 0700); e != nil {
		parent.close()
		return nil, e
	}
	fd, e := unix.Openat(int(parent.f.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		parent.close()
		return nil, e
	}
	d := &dir{os.NewFile(uintptr(fd), filepath.Join(base, name))}
	b := &UIBootstrap{parent: parent, session: d, name: name, expected: secret + "\n", Path: filepath.Join(base, name, "bootstrap")}
	if e = d.publish("bootstrap", []byte(b.expected), true); e != nil {
		b.Close()
		return nil, e
	}
	if e = parent.f.Sync(); e != nil {
		b.Close()
		return nil, e
	}
	return b, nil
}
func (b *UIBootstrap) consume() error {
	if b.consumed {
		return nil
	}
	actual, e := b.session.read("bootstrap")
	if e != nil {
		return e
	}
	if string(actual) != b.expected {
		return domain.E(domain.Conflict, "UI bootstrap file changed; preserving runtime evidence")
	}
	if e = unix.Unlinkat(int(b.session.f.Fd()), "bootstrap", 0); e != nil {
		return e
	}
	b.consumed = true
	b.expected = ""
	return b.session.f.Sync()
}
func (b *UIBootstrap) Consume() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return domain.E(domain.Unavailable, "UI runtime is closed")
	}
	return b.consume()
}
func (b *UIBootstrap) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	defer func() { b.closed = true; b.session.close(); b.parent.close() }()
	if e := b.consume(); e != nil {
		return e
	}
	if e := unix.Unlinkat(int(b.parent.f.Fd()), b.name, unix.AT_REMOVEDIR); e != nil {
		return e
	}
	return b.parent.f.Sync()
}
