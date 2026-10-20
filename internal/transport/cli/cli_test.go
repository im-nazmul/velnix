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
	for _, args := range [][]string{{"version", "--json"}, {"doctor", "--json"}, {"config", "show", "--json"}, {"config", "validate"}, {"state", "status", "--json"}, {"state", "init", "--dry-run", "--json"}, {"--help"}} {
		code, out, stderr := command(t, args...)
		if code != 0 || out == "" || stderr != "" {
			t.Fatalf("%v: %d %s %s", args, code, out, stderr)
		}
		if args[len(args)-1] == "--json" && !json.Valid([]byte(out)) {
			t.Fatal("invalid JSON output")
		}
	}
}
func TestErrors(t *testing.T) {
	for _, args := range [][]string{{"unknown", "--json"}, {"state"}, {"config", "nope", "--json"}, {"version", "unexpected"}, {"doctor", "--unknown"}, {"state", "init", "--dry-run", "--approve", "bad"}} {
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
