//go:build linux

package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIBootstrapPrivateConsumedAndSymlinkRefusal(t *testing.T) {
	base := t.TempDir()
	os.Chmod(base, 0700)
	b, e := CreateUIBootstrap(base, "ui-"+strings.Repeat("b", 32), strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(b.Path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("secret file is not private")
	}
	parent, _ := os.Stat(filepath.Dir(b.Path))
	if parent.Mode().Perm() != 0700 {
		t.Fatal("runtime directory is not private")
	}
	if e = b.Consume(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(b.Path); !os.IsNotExist(e) {
		t.Fatal("consumed secret remains")
	}
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Dir(b.Path)); !os.IsNotExist(e) {
		t.Fatal("empty runtime survives close")
	}
	alias := base + "-alias"
	os.Symlink(base, alias)
	defer os.Remove(alias)
	if b, e = CreateUIBootstrap(alias, "ui-"+strings.Repeat("c", 32), strings.Repeat("a", 64)); e == nil {
		b.Close()
		t.Fatal("symlink runtime accepted")
	}
	b, e = CreateUIBootstrap(base, "ui-"+strings.Repeat("d", 32), strings.Repeat("a", 64))
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(b.Path, []byte("changed"), 0600)
	if e = b.Close(); e == nil {
		t.Fatal("modified secret silently deleted")
	}
	if _, e = os.Stat(b.Path); e != nil {
		t.Fatal("changed evidence lost")
	}
}
