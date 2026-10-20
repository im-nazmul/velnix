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

func command(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	root := t.TempDir()
	paths := platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}
	c := CLI{Out: &out, Err: &stderr, Paths: paths, Detector: platform.Detector{Root: root}, Version: "test"}
	code := c.Run(context.Background(), args)
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("read-only command created state")
	}
	return code, out.String(), stderr.String()
}
func TestReadOnlyCommands(t *testing.T) {
	for _, args := range [][]string{{"version", "--json"}, {"doctor", "--json"}, {"config", "show", "--json"}, {"config", "validate"}, {"state", "status", "--json"}, {"state", "init", "--dry-run", "--json"}, {"projects", "list", "--json"}, {"http", "status", "--json"}, {"--help"}} {
		code, out, stderr := command(t, args...)
		if code != 0 || out == "" || stderr != "" {
			t.Fatalf("%v: %d %s %s", args, code, out, stderr)
		}
		if args[len(args)-1] == "--json" && !json.Valid([]byte(out)) {
			t.Fatal("invalid JSON output")
		}
	}
}

func TestMultiLanguageDiscoveryCLI(t *testing.T) {
	dir := t.TempDir()
	for _, kind := range []string{"node", "bun", "python"} {
		code, out, stderr := command(t, "runtime", "list", "--kind", kind, "--directory", dir, "--json")
		var got struct {
			Kind       string `json:"kind"`
			Candidates []any  `json:"candidates"`
			Issues     []any  `json:"issues"`
		}
		if code != 0 || stderr != "" || json.Unmarshal([]byte(out), &got) != nil || got.Kind != kind || got.Candidates == nil || got.Issues == nil {
			t.Fatalf("%s: %d %s %s", kind, code, out, stderr)
		}
	}
	for _, args := range [][]string{
		{"runtime", "list", "--kind", "ruby"},
		{"runtime", "list", "--kind", "node", "--directory", "relative"},
		{"runtime", "list", "--directory", dir},
		{"runtime", "resolve", "--kind", "node"},
	} {
		if code, _, _ := command(t, args...); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestProjectCLI(t *testing.T) {
	root := t.TempDir()
	paths := platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}
	var out, stderr bytes.Buffer
	c := CLI{Out: &out, Err: &stderr, Paths: paths}
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		stderr.Reset()
		if code := c.Run(context.Background(), args); code != 0 {
			t.Fatalf("%v: %d %s", args, code, stderr.String())
		}
		if !json.Valid(out.Bytes()) || stderr.Len() != 0 {
			t.Fatalf("invalid output: %s %s", out.String(), stderr.String())
		}
		return out.String()
	}
	planID := func(raw string) string {
		t.Helper()
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(raw), &p); err != nil || p.ID == "" {
			t.Fatalf("invalid plan: %s %v", raw, err)
		}
		return p.ID
	}
	initID := planID(run("state", "init", "--json"))
	run("state", "init", "--approve", initID, "--json")
	source := t.TempDir()
	linkID := planID(run("projects", "link", "--name", "demo", "--path", source, "--json"))
	if got := run("projects", "list", "--json"); got != "[]\n" {
		t.Fatal("preview linked project", got)
	}
	run("projects", "link", "--name", "demo", "--path", source, "--approve", linkID, "--json")
	var project struct {
		Root   string `json:"root"`
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal([]byte(run("projects", "inspect", "--name", "demo", "--json")), &project); err != nil || project.Root != source || project.Domain != "demo.test" {
		t.Fatalf("%+v %v", project, err)
	}
	unlinkID := planID(run("projects", "unlink", "--name", "demo", "--json"))
	run("projects", "unlink", "--name", "demo", "--approve", unlinkID, "--json")
	if _, err := os.Stat(source); err != nil {
		t.Fatal("source removed", err)
	}
}
func TestErrors(t *testing.T) {
	for _, args := range [][]string{{"unknown", "--json"}, {"state"}, {"config", "nope", "--json"}, {"version", "unexpected"}, {"doctor", "--unknown"}, {"state", "init", "--dry-run", "--approve", "bad"}, {"http", "start", "--dry-run", "--approve", "bad"}, {"http", "unknown", "--json"}} {
		code, out, stderr := command(t, args...)
		if code != 2 || out != "" || stderr == "" {
			t.Fatalf("%v: %d %q %q", args, code, out, stderr)
		}
	}
	code, _, stderr := command(t, "state", "init", "--approve", "bad", "--json")
	if code != 5 || !json.Valid([]byte(stderr)) {
		t.Fatalf("%d %s", code, stderr)
	}
}
func TestVersionWithoutHome(t *testing.T) {
	t.Setenv("HOME", "")
	var out, stderr bytes.Buffer
	c := CLI{Out: &out, Err: &stderr, Version: "test"}
	if code := c.Run(context.Background(), []string{"version"}); code != 0 {
		t.Fatalf("%d %s", code, stderr.String())
	}
}

func TestExplicitJSONBoolean(t *testing.T) {
	code, out, stderr := command(t, "version", "--json=true")
	if code != 0 || !json.Valid([]byte(out)) || stderr != "" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	code, out, stderr = command(t, "version", "--json=false")
	if code != 0 || out != "velnix test\n" || stderr != "" {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
}
