//go:build linux

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMailOwnershipLeaseAndExport(t *testing.T) {
	s := fixture(t)
	initStore(t, s)
	if disk, e := s.MailDisk(); e != nil || disk.Inode != 0 {
		t.Fatal(disk, e)
	}
	path := filepath.Join(s.Paths.Data, "mail")
	os.Mkdir(path, 0700)
	if _, e := s.MailData(false); e == nil {
		t.Fatal("unowned directory adopted")
	}
	os.Remove(path)
	m, e := s.MailData(true)
	if e != nil {
		t.Fatal(e)
	}
	m.Close()
	secret := filepath.Join(t.TempDir(), "external")
	os.WriteFile(secret, []byte("external"), 0600)
	os.Symlink(secret, filepath.Join(path, "mail.db"))
	if _, e = s.MailDisk(); e == nil {
		t.Fatal("symlink database accepted")
	}
	os.Remove(filepath.Join(path, "mail.db"))
	os.Link(secret, filepath.Join(path, "mail.db"))
	if _, e = s.MailDisk(); e == nil {
		t.Fatal("hardlinked database accepted")
	}
	os.Remove(filepath.Join(path, "mail.db"))
	first, e := s.MailLease()
	if e != nil {
		t.Fatal(e)
	}
	if second, e := s.MailLease(); e == nil {
		second.Close()
		t.Fatal("concurrent lease accepted")
	}
	first.Close()
	output := filepath.Join(t.TempDir(), "attachment")
	target, e := PlanMailExport(output)
	if e != nil {
		t.Fatal(e)
	}
	os.Symlink(secret, output)
	if e = WriteMailExport(target, []byte("overwrite")); e == nil {
		t.Fatal("symlink export accepted")
	}
	os.Remove(output)
	if e = WriteMailExport(target, []byte("owned")); e != nil {
		t.Fatal(e)
	}
	if e = WriteMailExport(target, []byte("overwrite")); e == nil {
		t.Fatal("export overwrite accepted")
	}
	b, _ := os.ReadFile(secret)
	if string(b) != "external" {
		t.Fatal("external file changed")
	}
	parent := filepath.Dir(output)
	alias := parent + "-alias"
	os.Symlink(parent, alias)
	defer os.Remove(alias)
	if _, e = PlanMailExport(filepath.Join(alias, "other")); e == nil {
		t.Fatal("symlink export parent accepted")
	}
}
