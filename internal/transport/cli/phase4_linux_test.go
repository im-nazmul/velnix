package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"velnix/internal/platform"
)

func TestPhase4CLIEndToEnd(t *testing.T) {
	python := "/usr/bin/python3"
	banner, e := exec.Command(python, "-I", "-S", "--version").Output()
	if e != nil {
		t.Skip("native Python required")
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(banner)), "Python ")
	root := t.TempDir()
	var out, stderr bytes.Buffer
	c := CLI{Out: &out, Err: &stderr, Paths: platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}
	run := func(args ...string) int { out.Reset(); stderr.Reset(); return c.Run(context.Background(), args) }
	plan := func(args ...string) string {
		t.Helper()
		if code := run(args...); code != 0 {
			t.Fatalf("%v %d %s", args, code, &stderr)
		}
		var p struct{ ID string }
		if e := json.Unmarshal(out.Bytes(), &p); e != nil || p.ID == "" {
			t.Fatalf("plan %s %v", &out, e)
		}
		return p.ID
	}
	id := plan("state", "init", "--json")
	if code := run("state", "init", "--approve", id); code != 0 {
		t.Fatal(code, &stderr)
	}
	project := filepath.Join(root, "project")
	os.Mkdir(project, 0700)
	os.WriteFile(filepath.Join(project, "pyproject.toml"), []byte("[project]\nrequires-python='>=3.10'\ndependencies=['fastapi']\n"), 0600)
	link := []string{"projects", "link", "--name", "demo", "--path", project, "--json"}
	id = plan(link...)
	if code := run(append(link, "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	selectArgs := []string{"runtime", "select", "--kind", "python", "--name", "demo", "--executable", python, "--version", version, "--json"}
	id = plan(selectArgs...)
	if code := run(append(selectArgs, "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	if code := run("runtime", "resolve", "--kind", "python", "--name", "demo", "--json"); code != 0 || !strings.Contains(out.String(), version) {
		t.Fatal(code, &out, &stderr)
	}
	if code := run("exec", "--kind", "python", "--name", "demo", "--", "-c", "import sys;print(sys.version_info.major);sys.exit(23)"); code != 23 || out.String() != "3\n" || stderr.Len() != 0 {
		t.Fatal(code, &out, &stderr)
	}
	if code := run("projects", "detect", "--name", "demo", "--json"); code != 0 || !strings.Contains(out.String(), "FastAPI") {
		t.Fatal(code, &out, &stderr)
	}
	proxy := []string{"projects", "proxy", "--name", "demo", "--port", "3000", "--json"}
	id = plan(proxy...)
	if code := run(append(proxy, "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	if code := run("runtime", "providers", "--json"); code != 0 || !strings.Contains(out.String(), "mise") || !strings.Contains(out.String(), "uv") {
		t.Fatal(code, &out, &stderr)
	}
	clear := []string{"runtime", "clear", "--kind", "python", "--name", "demo", "--json"}
	id = plan(clear...)
	if code := run(append(clear, "--approve", id)...); code != 0 {
		t.Fatal(code, &stderr)
	}
	if code := run("runtime", "resolve", "--kind", "python", "--name", "demo", "--json"); code == 0 {
		t.Fatal("cleared runtime resolved")
	}
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch != "" {
		store := filepath.Join(root, "uv-store")
		parts := strings.Split(version, ".")
		target := filepath.Join(store, "cpython-"+version+"-linux-"+arch+"-gnu", "bin", "python"+parts[0]+"."+parts[1])
		binary, e := os.ReadFile(python)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(target, binary, 0700); e != nil {
			t.Fatal(e)
		}
		locate := []string{"runtime", "locate", "--provider", "uv", "--provider-root", store, "--kind", "python", "--version", version, "--json"}
		if code := run(locate...); code != 0 {
			t.Fatal(code, &stderr)
		}
		var found struct {
			Provider, Authority, Layout string
			Probed                      bool
		}
		if e = json.Unmarshal(out.Bytes(), &found); e != nil || found.Provider != "uv" || found.Authority != "external" || found.Probed || found.Layout == "" {
			t.Fatalf("locate: %s %v", &out, e)
		}
		providerSelect := []string{"runtime", "select", "--provider", "uv", "--provider-root", store, "--kind", "python", "--name", "demo", "--version", version, "--json"}
		id = plan(providerSelect...)
		if !strings.Contains(out.String(), "provider_resolution") {
			t.Fatal("provider evidence omitted")
		}
		if code := run(append(providerSelect, "--approve", id)...); code != 0 {
			t.Fatal(code, &stderr)
		}
		if code := run("runtime", "resolve", "--kind", "python", "--name", "demo", "--json"); code != 0 || !strings.Contains(out.String(), "uv-store") {
			t.Fatal(code, &out, &stderr)
		}
		if code := run(append(providerSelect, "--executable", python)...); code == 0 {
			t.Fatal("provider and executable combined")
		}
	}
	for _, args := range [][]string{{"runtime", "select", "--kind", "python", "--name", "demo", "--php", python, "--version", version}, {"runtime", "select", "--kind", "php", "--name", "demo", "--executable", python, "--version", version}, {"exec", "--kind", "ruby", "--name", "demo", "--", "-v"}, {"projects", "proxy", "--name", "demo", "--port", "80"}} {
		if code := run(args...); code == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}
