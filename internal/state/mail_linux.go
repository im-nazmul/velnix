//go:build linux

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"velnix/internal/domain"
)

const mailOwner = "velnix-mail-v1\n"

// MailData pins a private provider directory. Only Velnix-created stores qualify.
type MailData struct{ d *dir }
type MailFile struct {
	Name     string `json:"name"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified_ns"`
}
type MailDisk struct {
	Device uint64     `json:"device"`
	Inode  uint64     `json:"inode"`
	Files  []MailFile `json:"files"`
	Bytes  int64      `json:"bytes"`
}

func (m *MailData) Close() error   { return m.d.f.Close() }
func (m *MailData) File() *os.File { return m.d.f }
func (m *MailData) Snapshot() (MailDisk, error) {
	var out MailDisk
	st, e := m.d.f.Stat()
	if e != nil {
		return out, e
	}
	raw := st.Sys().(*syscall.Stat_t)
	out.Device = uint64(raw.Dev)
	out.Inode = raw.Ino
	if _, e = m.d.f.Seek(0, 0); e != nil {
		return out, e
	}
	names, e := m.d.f.Readdirnames(-1)
	if e != nil {
		return out, e
	}
	sort.Strings(names)
	for _, name := range names {
		switch name {
		case "owner", "mail.db", "mail.db-wal", "mail.db-shm":
		default:
			return out, domain.E(domain.Conflict, "unrecognized file in owned mail store")
		}
		f, e := m.d.open(name, unix.O_RDONLY)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return out, e
		}
		info, e := f.Stat()
		f.Close()
		if e != nil {
			return out, e
		}
		s := info.Sys().(*syscall.Stat_t)
		out.Files = append(out.Files, MailFile{name, uint64(s.Dev), s.Ino, info.Size(), info.ModTime().UnixNano()})
		out.Bytes += info.Size()
	}
	return out, nil
}
func (s Store) MailData(create bool) (*MailData, error) {
	parent, e := openDir(s.Paths.Data, false)
	if e != nil {
		return nil, e
	}
	defer parent.close()
	if create {
		if e = unix.Mkdirat(int(parent.f.Fd()), "mail", 0700); e != nil {
			return nil, e
		}
		if e = parent.f.Sync(); e != nil {
			return nil, e
		}
	}
	fd, e := unix.Openat(int(parent.f.Fd()), "mail", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	d := &dir{os.NewFile(uintptr(fd), filepath.Join(s.Paths.Data, "mail"))}
	fail := func(e error) (*MailData, error) { d.close(); return nil, e }
	st, e := d.f.Stat()
	if e != nil {
		return fail(e)
	}
	raw := st.Sys().(*syscall.Stat_t)
	if st.Mode().Perm() != 0700 || raw.Uid != uint32(os.Geteuid()) {
		return fail(domain.E(domain.Denied, "unsafe mail directory"))
	}
	if create {
		if e = d.publish("owner", []byte(mailOwner), true); e != nil {
			return fail(e)
		}
	}
	b, e := d.read("owner")
	if e != nil {
		return fail(e)
	}
	if string(b) != mailOwner {
		return fail(domain.E(domain.Conflict, "mail directory lacks Velnix ownership evidence"))
	}
	var filesystem unix.Statfs_t
	if e = unix.Fstatfs(int(d.f.Fd()), &filesystem); e != nil {
		return fail(e)
	}
	switch uint64(filesystem.Type) {
	case 0x6969, 0xff534d42, 0x517b, 0x01021997, 0x65735546:
		return fail(domain.E(domain.Unavailable, "mail storage requires a local Linux filesystem; network, 9p and FUSE stores are unsupported"))
	}
	m := &MailData{d}
	if _, e = m.Snapshot(); e != nil {
		return fail(e)
	}
	return m, nil
}
func (s Store) MailDisk() (MailDisk, error) {
	m, e := s.MailData(false)
	if errors.Is(e, os.ErrNotExist) {
		return MailDisk{}, nil
	}
	if e != nil {
		return MailDisk{}, e
	}
	defer m.Close()
	return m.Snapshot()
}

// MailLease is independent of the global mutation lock and lasts for one capture.
func (s Store) MailLease() (*os.File, error) {
	d, e := openDir(s.Paths.State, false)
	if e != nil {
		return nil, e
	}
	defer d.close()
	f, e := d.open("mail.lock", unix.O_RDWR|unix.O_CREAT)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, domain.E(domain.Conflict, "another mail capture session holds the lifetime lock")
	}
	return f, nil
}

// MailRuntime is ephemeral private storage, with a validated UID fallback.
func MailRuntimeBase() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base != "" {
		d, e := openDir(base, false)
		if e != nil {
			return "", e
		}
		d.close()
		return base, nil
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("velnix-mail-%d", os.Geteuid())), nil
}
func validMailSession(path string) bool {
	name := filepath.Base(path)
	return strings.HasPrefix(name, "mail-") && len(name) == 37 && strings.Trim(name[5:], "0123456789abcdef") == ""
}
func CreateMailSession(base, name string, credentials []byte) (string, error) {
	if !validMailSession(name) {
		return "", domain.E(domain.Invalid, "invalid mail session name")
	}
	d, e := openDir(base, true)
	if e != nil {
		return "", e
	}
	defer d.close()
	if e = unix.Mkdirat(int(d.f.Fd()), name, 0700); e != nil {
		return "", e
	}
	if e = d.f.Sync(); e != nil {
		return "", e
	}
	path := filepath.Join(base, name)
	session, e := openDir(path, false)
	if e != nil {
		return "", e
	}
	defer session.close()
	if e = session.publish("credentials.json", credentials, true); e != nil {
		return "", e
	}
	return path, nil
}
func ReadMailCredentials(path string) ([]byte, error) {
	if !validMailSession(path) {
		return nil, domain.E(domain.Invalid, "invalid mail session")
	}
	return readAt(path, "credentials.json")
}
func RemoveMailCredentials(path string) error {
	if !validMailSession(path) {
		return domain.E(domain.Invalid, "invalid mail session")
	}
	d, e := openDir(path, false)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	defer d.close()
	f, e := d.open("credentials.json", unix.O_RDONLY)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	f.Close()
	if e = unix.Unlinkat(int(d.f.Fd()), "credentials.json", 0); e != nil {
		return e
	}
	if e = d.f.Sync(); e != nil {
		return e
	}
	parent, e := openDir(filepath.Dir(path), false)
	if e != nil {
		return e
	}
	defer parent.close()
	if e = unix.Unlinkat(int(parent.f.Fd()), filepath.Base(path), unix.AT_REMOVEDIR); e != nil {
		return e
	}
	return parent.f.Sync()
}

type ExportLocation struct {
	Path   string `json:"path"`
	Device uint64 `json:"parent_device"`
	Inode  uint64 `json:"parent_inode"`
}

func exportParent(path string) (*dir, ExportLocation, error) {
	var out ExportLocation
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." || path == "/" {
		return nil, out, domain.E(domain.Invalid, "attachment output must be a clean absolute file path")
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, out, e
	}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return nil, out, err
		}
		fd = next
	}
	d := &dir{os.NewFile(uintptr(fd), filepath.Dir(path))}
	fail := func(e error) (*dir, ExportLocation, error) { d.close(); return nil, out, e }
	info, e := d.f.Stat()
	if e != nil {
		return fail(e)
	}
	st := info.Sys().(*syscall.Stat_t)
	if st.Uid != uint32(os.Geteuid()) || info.Mode().Perm()&0022 != 0 {
		return fail(domain.E(domain.Denied, "export parent must be current-user owned without group/world write"))
	}
	var target unix.Stat_t
	e = unix.Fstatat(fd, filepath.Base(path), &target, unix.AT_SYMLINK_NOFOLLOW)
	if e == nil {
		return fail(domain.E(domain.Conflict, "export destination already exists"))
	}
	if !errors.Is(e, unix.ENOENT) {
		return fail(e)
	}
	out = ExportLocation{path, uint64(st.Dev), st.Ino}
	return d, out, nil
}
func PlanMailExport(path string) (ExportLocation, error) {
	d, out, e := exportParent(path)
	if d != nil {
		d.close()
	}
	return out, e
}
func WriteMailExport(target ExportLocation, b []byte) error {
	d, now, e := exportParent(target.Path)
	if e != nil {
		return e
	}
	defer d.close()
	if now != target {
		return domain.E(domain.Conflict, "export parent identity changed")
	}
	return d.publish(filepath.Base(target.Path), b, true)
}
