//go:build linux

package managedstore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func copyNative(t *testing.T, path string) {
	t.Helper()
	b, e := os.ReadFile("/bin/true")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0700); e != nil {
		t.Fatal(e)
	}
}
func uvEntry(version string) string {
	return "cpython-" + version + "-linux-" + map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH] + "-gnu"
}
func TestSupportedExistingLayoutsWithoutExecution(t *testing.T) {
	for _, tt := range []struct{ provider, kind, version, entry string }{{"mise", "node", "25.9.0", "installs/node/25.9.0/bin/node"}, {"mise", "bun", "1.4.3", "installs/bun/1.4.3/bin/bun"}, {"uv", "python", "3.12.3", uvEntry("3.12.3") + "/bin/python3.12"}} {
		t.Run(tt.provider+tt.kind, func(t *testing.T) {
			root := t.TempDir()
			candidate := filepath.Join(root, tt.entry)
			copyNative(t, candidate)
			got, e := Resolve(Request{Provider: tt.provider, Root: root, Kind: tt.kind, Version: tt.version})
			if e != nil || got.Executable.Path != candidate || got.Probed || got.Authority != "external" || got.Origin != "unknown" || got.Layout != Layout {
				t.Fatalf("%+v %v", got, e)
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Fatal("resolution created files")
			}
		})
	}
}
func TestMiseIdentityStoreAndEscapes(t *testing.T) {
	root := t.TempDir()
	identity := filepath.Join(root, "i", "node-identity")
	copyNative(t, filepath.Join(identity, "bin", "node"))
	os.MkdirAll(filepath.Join(root, "installs", "node"), 0700)
	link := filepath.Join(root, "installs", "node", "25.9.0")
	os.Symlink(identity, link)
	req := Request{Provider: "mise", Root: root, Kind: "node", Version: "25.9.0"}
	got, e := Resolve(req)
	if e != nil || got.Executable.Path != filepath.Join(identity, "bin", "node") {
		t.Fatalf("identity store: %+v %v", got, e)
	}
	// A new exact-version alias cannot silently escape the reviewed storage root.
	outside := t.TempDir()
	copyNative(t, filepath.Join(outside, "bin", "node"))
	os.Remove(link)
	os.Symlink(outside, link)
	if _, e = Resolve(req); e == nil {
		t.Fatal("external installation store accepted")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	os.Symlink("/", alias)
	req.Root = alias
	if _, e = Resolve(req); e == nil {
		t.Fatal("canonical / accepted")
	}
}
func TestInvalidAndUnsupportedStores(t *testing.T) {
	root := t.TempDir()
	for _, req := range []Request{{"mise", root, "node", "25"}, {"mise", "relative", "node", "25.9.0"}, {"mise", root, "python", "3.12.3"}, {"uv", root, "node", "25.9.0"}, {"other", root, "node", "25.9.0"}, {"uv", root, "python", "3.12.3"}, {"mise", root, "node", "25.9.0"}} {
		if _, e := Resolve(req); e == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	path := filepath.Join(root, "installs", "node", "25.9.0", "bin", "node")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("#!/bin/sh\ntouch should-not-run\n"), 0700)
	if _, e := Resolve(Request{"mise", root, "node", "25.9.0"}); e == nil {
		t.Fatal("shim accepted")
	}
	if _, e := os.Stat(filepath.Join(root, "should-not-run")); !os.IsNotExist(e) {
		t.Fatal("shim executed")
	}
}
func TestUVBoundAndForeignVariants(t *testing.T) {
	root := t.TempDir()
	copyNative(t, filepath.Join(root, "cpython-3.12.3-linux-foreign-gnu", "bin", "python3.12"))
	if _, e := Resolve(Request{"uv", root, "python", "3.12.3"}); e == nil {
		t.Fatal("foreign variant selected")
	}
	// Use guaranteed distinct numeric names for an over-limit store.
	for i := 0; i < 1025; i++ {
		os.Mkdir(filepath.Join(root, fmt.Sprintf("entry-%04d", i)), 0700)
	}
	_, e := Resolve(Request{"uv", root, "python", "3.12.3"})
	if e == nil || !strings.Contains(e.Error(), "1024") {
		t.Fatalf("unbounded store accepted: %v", e)
	}
}
