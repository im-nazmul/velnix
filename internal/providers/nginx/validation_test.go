package nginx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealValidationPreservesWorkingConfig(t *testing.T) {
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("installed Nginx required")
	}
	prefix := t.TempDir()
	working := filepath.Join(prefix, "http.conf")
	original := []byte("# prior working generation\n")
	if e = os.WriteFile(working, original, 0600); e != nil {
		t.Fatal(e)
	}
	candidate := filepath.Join(prefix, "candidate.conf")
	if e = os.WriteFile(candidate, []byte("invalid_directive;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = Validate(ctx, exe, prefix, candidate); e == nil {
		t.Fatal("accepted invalid config")
	}
	b, e := os.ReadFile(working)
	if e != nil || string(b) != string(original) {
		t.Fatal("validation replaced existing config")
	}
}
