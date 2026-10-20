package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/platform"
	"velnix/internal/providers/phpcli"
)

func TestRuntimeCLI(t *testing.T) {
	path, version := "", ""
	for _, p := range []string{"/usr/local/bin/php", "/usr/bin/php"} {
		if _, err := phpcli.Snapshot(p, "8.3.12"); err != nil {
			continue
		}
		c := exec.Command(p, "-n", "-r", "echo PHP_VERSION;")
		c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
		b, err := c.Output()
		if err != nil {
			t.Fatal(err)
		}
		path, version = p, string(b)
		break
	}
	if path == "" {
		t.Skip("native PHP CLI not installed")
	}
	root := t.TempDir()
	var out, stderr bytes.Buffer
	c := CLI{In: strings.NewReader("stdin"), Out: &out, Err: &stderr, Paths: platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}
	run := func(args ...string) int {
		t.Helper()
		out.Reset()
		stderr.Reset()
		return c.Run(context.Background(), args)
	}
	plan := func(args ...string) string {
		t.Helper()
		if code := run(args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out.Bytes(), &p); err != nil || p.ID == "" {
			t.Fatalf("bad plan %s %v", &out, err)
		}
		return p.ID
	}
	id := plan("runtime", "inspect", "--php", path, "--version", version, "--json")
	if code := run("runtime", "inspect", "--php", path, "--version", version, "--approve", id, "--json"); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatalf("probe: %d %s", code, &stderr)
	}
	id = plan("state", "init", "--json")
	if code := run("state", "init", "--approve", id); code != 0 {
		t.Fatal(code, &stderr)
	}
	id = plan("projects", "link", "--name", "demo", "--path", t.TempDir(), "--json")
	// Repeat link uses the project root from the plan's target registry.
	var link struct {
		Registry struct {
			Projects []struct {
				Root string `json:"root"`
			} `json:"projects"`
		} `json:"registry_target"`
	}
	if err := json.Unmarshal(out.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if code := run("projects", "link", "--name", "demo", "--path", link.Registry.Projects[0].Root, "--approve", id); code != 0 {
		t.Fatal(code, &stderr)
	}
	base := []string{"runtime", "select", "--name", "demo", "--php", path, "--version", version}
	id = plan(append(append([]string{}, base...), "--json")...)
	if code := run(append(append([]string{}, base...), "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	if code := run("runtime", "resolve", "--name", "demo", "--json"); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatal(code, &out, &stderr)
	}
	if code := run("exec", "--name", "demo", "--", "-n", "-r", `echo $argv[1], stream_get_contents(STDIN); exit(23);`, "--", "--json"); code != 23 || out.String() != "--jsonstdin" || stderr.Len() != 0 {
		t.Fatalf("child output/status: %d %s %s", code, &out, &stderr)
	}
	if code := run("exec", "--name", "demo", "-r", "echo 1;"); code != 2 {
		t.Fatalf("missing separator accepted: %d", code)
	}
	if code := run("exec", "--json", "--name", "demo", "--", "-v"); code != 2 {
		t.Fatalf("exec JSON accepted: %d", code)
	}
}
