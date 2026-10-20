//go:build linux

package runtimeinventory

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"velnix/internal/providers/nativeexec"
)

func probeFixture(t *testing.T) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler unavailable for native probe fixture")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "fixture.c")
	code := `#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
int main(int argc, char **argv) {
 char cwd[4096];
 if (!getcwd(cwd, sizeof(cwd)) || strcmp(cwd, "/") || getenv("LD_PRELOAD") || getenv("NODE_OPTIONS") || getenv("PYTHONPATH") || getchar() != EOF) return 9;
 const char *name = strrchr(argv[0], '/'); name = name ? name+1 : argv[0];
 if (!strcmp(name,"slow")) { sleep(20); return 0; }
 if (!strcmp(name,"noise")) { for (int i=0;i<10000;i++) putchar('x'); return 0; }
 if (!strcmp(name,"stderr")) { fprintf(stderr,"diagnostic"); puts("1.2.3"); return 0; }
 if (!strcmp(name,"failure")) return 8;
 if (!strcmp(name,"node")) { if(argc!=2 || strcmp(argv[1],"--version")) return 7; puts("v1.2.3"); }
 else if (!strcmp(name,"python")) { if(argc!=4 || strcmp(argv[1],"-I") || strcmp(argv[2],"-S") || strcmp(argv[3],"--version")) return 7; puts("Python 1.2.3"); }
 else puts("1.2.3");
 return 0;
}`
	if err = os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command(cc, source, "-o", filepath.Join(dir, "fixture")).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	for _, name := range []string{"node", "bun", "python", "slow", "noise", "stderr", "failure"} {
		if err = os.Link(filepath.Join(dir, "fixture"), filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestProbeIsolationVersionAndFailures(t *testing.T) {
	dir := probeFixture(t)
	t.Setenv("NODE_OPTIONS", "--require=/nonexistent/velnix.js")
	t.Setenv("PYTHONPATH", "/nonexistent/velnix")
	t.Setenv("LD_PRELOAD", "/nonexistent/velnix.so")
	for _, kind := range []string{"node", "bun", "python"} {
		s, err := nativeexec.Snapshot(filepath.Join(dir, kind), "1.2.3")
		if err != nil {
			t.Fatal(err)
		}
		if err = Probe(context.Background(), kind, s); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		s.Version = "9.9.9"
		if err = Probe(context.Background(), kind, s); err == nil {
			t.Fatal("mismatched version accepted")
		}
	}
	for _, name := range []string{"noise", "stderr", "failure"} {
		s, err := nativeexec.Snapshot(filepath.Join(dir, name), "1.2.3")
		if err != nil {
			t.Fatal(err)
		}
		if err = Probe(context.Background(), "bun", s); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	s, err := nativeexec.Snapshot(filepath.Join(dir, "slow"), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err = Probe(context.Background(), "bun", s); !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 5*time.Second {
		t.Fatalf("timeout failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = Probe(ctx, "bun", s); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = Probe(context.Background(), "ruby", s); err == nil {
		t.Fatal("unsupported kind accepted")
	}
}

func TestInstalledRuntimeVersionProbes(t *testing.T) {
	for _, kind := range []string{"node", "bun", "python"} {
		t.Run(kind, func(t *testing.T) {
			name := kind
			if kind == "python" {
				name = "python3"
			}
			path, err := exec.LookPath(name)
			if err != nil {
				t.Skip("runtime not installed")
			}
			// Reject shims before using a host executable in this integration test.
			if _, err = nativeexec.Snapshot(path, "0.0.0"); err != nil {
				t.Skipf("unsupported native candidate: %v", err)
			}
			args, _ := ProbeArgs(kind)
			c := exec.Command(path, args...)
			c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
			c.Dir = "/"
			b, err := c.Output()
			if err != nil {
				t.Fatal(err)
			}
			version := strings.TrimSpace(string(b))
			if kind == "node" {
				version = strings.TrimPrefix(version, "v")
			}
			if kind == "python" {
				version = strings.TrimPrefix(version, "Python ")
			}
			s, err := nativeexec.Snapshot(path, version)
			if err != nil {
				t.Fatal(err)
			}
			if err = Probe(context.Background(), kind, s); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %s", s.Path, version)
		})
	}
}
