package runtimeinventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoveryIsReadOnlyAndDeduplicatesAliases(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	for _, name := range []string{"node", "bun", "python3.12", "python-config", "node_modules"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "node"), filepath.Join(dir, "nodejs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "python3.12"), filepath.Join(dir, "python3")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "python2"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "python4"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "python5")); err != nil {
		t.Fatal(err)
	}
	for kind, name := range map[string]string{"node": "node", "bun": "bun", "python": "python3.12"} {
		got, err := Discover(kind, []string{dir, dir})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Candidates) != 1 || len(got.Issues) != 0 {
			t.Fatalf("%+v", got)
		}
		c := got.Candidates[0]
		if c.Path != filepath.Join(dir, name) || c.Origin != "unknown" || c.Authority != "external" || c.Probed {
			t.Fatalf("%+v", c)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("discovery executed candidate")
	}
}

func TestInvalidRequestsAndUnavailableDirectory(t *testing.T) {
	for _, tc := range []struct{ kind, dir string }{{"ruby", "/usr/bin"}, {"node", "relative"}, {"bun", "/usr/../bin"}} {
		if _, err := Discover(tc.kind, []string{tc.dir}); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	missing := filepath.Join(t.TempDir(), "absent")
	got, err := Discover("python", []string{missing})
	if err != nil || len(got.Candidates) != 0 || len(got.Issues) != 1 || got.Issues[0].Reason != "unavailable" {
		t.Fatalf("%+v %v", got, err)
	}
}
