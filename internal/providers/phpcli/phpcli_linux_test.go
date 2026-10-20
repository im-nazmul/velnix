package phpcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryAndSnapshotsNeverExecuteShims(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	shim := filepath.Join(dir, "php")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shim, filepath.Join(dir, "php8.3")); err != nil {
		t.Fatal(err)
	}
	candidates := DiscoverIn([]string{dir})
	if len(candidates) != 1 || candidates[0].Authority != "external" || candidates[0].Probed {
		t.Fatalf("%+v", candidates)
	}
	if _, err := Snapshot(shim, "8.3.12"); err == nil {
		t.Fatal("shell shim accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("shim executed")
	}
}

func TestExecutableIdentityAndPinnedDescriptor(t *testing.T) {
	b, err := os.ReadFile("/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "php")
	if err = os.WriteFile(path, b, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Snapshot(path, "8.3.12")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("#!/bin/sh\nexit 9"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(s); err == nil {
		t.Fatal("replacement accepted")
	}
	if err = Command(f, s).Run(); err != nil {
		t.Fatal("pinned original was not executed", err)
	}
	if err = os.WriteFile(path, b, 0777); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err = Snapshot(path, "8.3.12"); err == nil {
		t.Fatal("writable binary accepted")
	}
	if _, err = Snapshot("php", "8.3.12"); err == nil {
		t.Fatal("relative executable accepted")
	}
}

func TestRealPHPProbeExactVersionAndControlledEnvironment(t *testing.T) {
	path := ""
	for _, p := range []string{"/usr/local/bin/php", "/usr/bin/php"} {
		if _, err := Snapshot(p, "8.3.12"); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Skip("native PHP CLI not installed")
	}
	c := exec.Command(path, "-n", "-r", "echo PHP_VERSION;")
	c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	b, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	version := strings.TrimSpace(string(b))
	t.Logf("real PHP: %s %s", path, version)
	s, err := Snapshot(path, version)
	if err != nil {
		t.Fatal(err)
	}
	ini := t.TempDir()
	if err = os.WriteFile(filepath.Join(ini, "php.ini"), []byte("extension=/nonexistent/velnix-test.so\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHPRC", ini)
	t.Setenv("PHP_INI_SCAN_DIR", ini)
	t.Setenv("LD_PRELOAD", "/nonexistent/velnix-test.so")
	if err = Probe(context.Background(), s); err != nil {
		t.Fatal("probe inherited unsafe environment or loaded ini", err)
	}
	s.Version = "0.0.0"
	if err = Probe(context.Background(), s); err == nil {
		t.Fatal("wrong exact version accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = Probe(ctx, s); err == nil {
		t.Fatal("canceled probe succeeded")
	}
}

func TestBoundedProbeOutput(t *testing.T) {
	var b bounded
	p := []byte(strings.Repeat("x", 20000))
	if n, err := b.Write(p); err != nil || n != len(p) || b.Len() != 4096 || !b.overflow {
		t.Fatal("output was not bounded")
	}
	if _, err := b.Write(p); err != nil || b.Len() != 4096 {
		t.Fatal("continued output was not drained")
	}
}
