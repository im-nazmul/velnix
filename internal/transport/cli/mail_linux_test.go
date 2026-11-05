//go:build linux

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

func TestMailCLIReadOnlyAndInvalidArguments(t *testing.T) {
	root := t.TempDir()
	var out, stderr bytes.Buffer
	c := CLI{Out: &out, Err: &stderr, Paths: platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}
	if code := c.Run(context.Background(), []string{"mail", "status", "--json"}); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var status map[string]any
	if e := json.Unmarshal(out.Bytes(), &status); e != nil || status["available"] != false {
		t.Fatal(out.String(), e)
	}
	files, _ := os.ReadDir(root)
	if len(files) != 0 {
		t.Fatal("status initialized state")
	}
	for _, args := range [][]string{{"mail", "list", "--json"}, {"mail", "show"}, {"mail", "unknown"}, {"mail", "status", "--approve", "bad"}, {"mail", "serve", "--dry-run", "--approve", "bad"}, {"mail", "show", "../unsafe", "--json"}} {
		out.Reset()
		stderr.Reset()
		if code := c.Run(context.Background(), args); code == 0 {
			t.Fatal("invalid/unavailable request accepted", args)
		}
	}
	out.Reset()
	stderr.Reset()
	if code := c.Run(context.Background(), []string{"mail", "--help"}); code != 0 || out.Len() == 0 {
		t.Fatal(code, stderr.String())
	}
}
