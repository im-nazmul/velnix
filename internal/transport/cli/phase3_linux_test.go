package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"velnix/internal/platform"
)

func TestPhase3CLIPlans(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged workloads required")
	}
	root := t.TempDir()
	var out, stderr bytes.Buffer
	c := CLI{Out: &out, Err: &stderr, Paths: platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}
	run := func(args ...string) int { out.Reset(); stderr.Reset(); return c.Run(context.Background(), args) }
	plan := func(args ...string) string {
		t.Helper()
		if code := run(args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
		var p struct{ ID string }
		if e := json.Unmarshal(out.Bytes(), &p); e != nil || p.ID == "" {
			t.Fatalf("plan: %s %v", &out, e)
		}
		return p.ID
	}
	id := plan("state", "init", "--json")
	if code := run("state", "init", "--approve", id); code != 0 {
		t.Fatal(code, &stderr)
	}
	project := filepath.Join(root, "project")
	os.Mkdir(project, 0700)
	args := []string{"projects", "link", "--name", "demo", "--path", project, "--json"}
	id = plan(args...)
	if code := run(append(args, "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	// /bin/true is native and preview never invokes it as a runtime/mkcert.
	fpm := []string{"fpm", "start", "--name", "demo", "--php-fpm", "/bin/true", "--version", "1.2.3", "--profile", "front-controller", "--uploads", "uploads,files", "--dry-run", "--json"}
	plan(fpm...)
	tls := []string{"tls", "issue", "--mkcert", "/bin/true", "--json"}
	id = plan(tls...)
	if code := run(append(tls, "--approve", id)...); code == 0 {
		t.Fatal("non-mkcert accepted")
	}
	for _, args := range [][]string{{"fpm", "status", "--name", "../escape"}, {"fpm", "start", "--name", "demo", "--php-fpm", "/bin/true", "--version", "1.2.3", "--dry-run", "--approve", "wrong"}, {"tls", "issue", "--dry-run", "--approve", "wrong"}, {"tls", "issue", "--install"}, {"tls", "verify", "--name", "demo", "--port", "443"}} {
		if code := run(args...); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, name := range []string{"fpm-demo-http.json", "tls.json"} {
		if _, e := os.Stat(filepath.Join(c.Paths.State, name)); !os.IsNotExist(e) {
			t.Fatalf("preview/failure created %s", name)
		}
	}
}
