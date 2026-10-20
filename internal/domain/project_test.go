package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectPathsAndIdentity(t *testing.T) {
	root := t.TempDir()
	public := filepath.Join(root, "public")
	if err := os.Mkdir(public, 0755); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveProject("demo", root, "public")
	if err != nil || p.Root != root || p.DocumentRoot != public || p.Domain != "demo.test" {
		t.Fatalf("%+v %v", p, err)
	}
	outside := t.TempDir()
	if err = os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{"escape", outside, "../"} {
		if _, err = ResolveProject("demo", root, doc); err == nil {
			t.Fatalf("accepted escaping document root: %s", doc)
		}
	}
	if err = os.Rename(public, public+"-old"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(public, 0755); err != nil {
		t.Fatal(err)
	}
	replacement, err := ResolveProject("demo", root, "public")
	if err != nil || replacement.DocumentInode == p.DocumentInode {
		t.Fatalf("replacement identity not detected: %+v %v", replacement, err)
	}
}

func TestProjectNames(t *testing.T) {
	for _, name := range []string{"", "Demo", "-demo", "demo-", "demo.test", "../demo", "demo\n"} {
		if ValidProjectName(name) {
			t.Fatalf("accepted invalid name %q", name)
		}
	}
}
