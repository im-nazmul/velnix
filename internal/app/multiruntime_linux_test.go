//go:build linux

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/runtimeinventory"
	"velnix/internal/state"
)

func installed(t *testing.T, kind string) (string, string) {
	t.Helper()
	name := kind
	if kind == "python" {
		name = "python3"
	}
	path, e := exec.LookPath(name)
	if e != nil {
		t.Skip("installed " + kind + " required")
	}
	path, e = filepath.EvalSymlinks(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = nativeexec.Snapshot(path, "0.0.0"); e != nil {
		t.Skipf("native %s unavailable: %v", kind, e)
	}
	args, _ := runtimeinventory.ProbeArgs(kind)
	cmd := exec.Command(path, args...)
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	b, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	v := strings.TrimSpace(string(b))
	if kind == "node" {
		v = strings.TrimPrefix(v, "v")
	}
	if kind == "python" {
		v = strings.TrimPrefix(v, "Python ")
	}
	return path, v
}
func selectMulti(t *testing.T, s RuntimeService, r RuntimeRequest) {
	t.Helper()
	plan, e := s.Run(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	r.Approval = plan.(state.ProjectPlan).ID
	if _, e = s.Run(context.Background(), r); e != nil {
		t.Fatal(e)
	}
}
func TestRealMultiLanguageSelectionAndExec(t *testing.T) {
	for _, kind := range []string{"node", "bun", "python"} {
		t.Run(kind, func(t *testing.T) {
			path, version := installed(t, kind)
			http := httpFixture(t)
			s := RuntimeService{Store: http.Store}
			st, _ := http.Store.Status()
			root := st.Registry.Projects[0].Root
			t.Setenv("NODE_OPTIONS", "--require /missing/untrusted-project-hook")
			t.Setenv("BUN_OPTIONS", "--preload /missing/untrusted-project-hook")
			t.Setenv("PYTHONPATH", "/missing/untrusted")
			t.Setenv("PYTHONHOME", "/missing/untrusted")
			t.Setenv("VIRTUAL_ENV", "/stale/environment")
			r := RuntimeRequest{Action: "select", Name: "one", Kind: kind, Path: path, Version: version}
			preview, e := s.Run(context.Background(), r)
			if e != nil {
				t.Fatal(e)
			}
			if st, _ := http.Store.Status(); st.Registry.Schema != 2 {
				t.Fatal("preview migrated state")
			}
			r.Approval = preview.(state.ProjectPlan).ID
			if _, e = s.Run(context.Background(), r); e != nil {
				t.Fatal(e)
			}
			resolved, e := s.ResolveKind("one", kind)
			if e != nil || resolved.Executable.Path != path || resolved.Directory != root {
				t.Fatalf("resolve %+v %v", resolved, e)
			}
			script := `console.log(JSON.stringify({version:process.versions.node,cwd:process.cwd(),options:process.env.NODE_OPTIONS??null,argv:process.argv.slice(1)}))`
			args := []string{"-e", script, "--", "literal;$(touch nope)"}
			if kind == "bun" {
				args = []string{"-e", `console.log(JSON.stringify({version:Bun.version,cwd:process.cwd(),options:process.env.BUN_OPTIONS??null,argv:process.argv.slice(1)}))`, "--", "literal;$(touch nope)"}
			}
			if kind == "python" {
				args = []string{"-c", `import sys,os,json;print(json.dumps(dict(version='.'.join(map(str,sys.version_info[:3])),cwd=os.getcwd(),options=os.environ.get('PYTHONPATH'),argv=sys.argv[1:])))`, "literal;$(touch nope)"}
			}
			var out bytes.Buffer
			var stderr bytes.Buffer
			if e = s.ExecKind(context.Background(), "one", kind, args, nil, &out, &stderr); e != nil {
				t.Fatalf("exec: %v %s", e, &stderr)
			}
			var got struct {
				Version, Cwd string
				Options      *string
				Argv         []string
			}
			if e = json.Unmarshal(out.Bytes(), &got); e != nil || got.Version != version || got.Cwd != root || got.Options != nil || len(got.Argv) == 0 || got.Argv[len(got.Argv)-1] != "literal;$(touch nope)" {
				t.Fatalf("output %s %v", &out, e)
			}
			if os.Getenv("PYTHONHOME") != "/missing/untrusted" {
				t.Fatal("parent environment changed")
			}
			exitArgs := []string{"-e", "process.exit(23)"}
			if kind == "python" {
				exitArgs = []string{"-c", "import sys;sys.exit(23)"}
			}
			e = s.ExecKind(context.Background(), "one", kind, exitArgs, nil, &out, &stderr)
			var child *ChildExit
			if !errors.As(e, &child) || child.Code != 23 {
				t.Fatalf("exit status: %v", e)
			}
			change := state.ProjectChange{Action: "link", Name: "one", Root: root, DocumentRoot: "."}
			p, e := http.Store.PlanProject(change)
			if e != nil || p.Registry.Projects[0].Runtime(kind).Executable.Path != path {
				t.Fatalf("relink lost selection: %v", e)
			}
			clear := RuntimeRequest{Action: "clear", Name: "one", Kind: kind}
			selectMulti(t, s, clear)
			if _, e = s.ResolveKind("one", kind); e == nil {
				t.Fatal("cleared runtime resolved")
			}
		})
	}
}
func TestPythonVirtualenvSelection(t *testing.T) {
	path, version := installed(t, "python")
	http := httpFixture(t)
	s := RuntimeService{Store: http.Store}
	st, _ := http.Store.Status()
	root := st.Registry.Projects[0].Root
	venv := filepath.Join(root, ".venv")
	command := exec.Command(path, "-I", "-m", "venv", "--without-pip", venv)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	if b, e := command.CombinedOutput(); e != nil {
		t.Skipf("venv creation unavailable: %v %s", e, b)
	}
	// Project hook is never evaluated by preview, version probing, or resolve.
	marker := filepath.Join(root, "hook-ran")
	os.WriteFile(filepath.Join(root, "sitecustomize.py"), []byte("open("+string(data(marker))+",'w').write('bad')"), 0600)
	req := RuntimeRequest{Action: "select", Name: "one", Kind: "python", Path: path, Version: version}
	selectMulti(t, s, req)
	resolved, e := s.ResolveKind("one", "python")
	if e != nil || resolved.Environment["VIRTUAL_ENV"] != venv {
		t.Fatalf("venv resolution %+v %v", resolved, e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("selection executed a project hook")
	}
	var out, stderr bytes.Buffer
	e = s.ExecKind(context.Background(), "one", "python", []string{"-c", `import sys,json;print(json.dumps([sys.prefix,sys.base_prefix,sys.executable]))`}, nil, &out, &stderr)
	var observed []string
	if e != nil || json.Unmarshal(out.Bytes(), &observed) != nil || len(observed) != 3 || observed[0] != venv || observed[0] == observed[1] {
		t.Fatalf("venv execution %s %s %v", &out, &stderr, e)
	}
	cfg := filepath.Join(venv, "pyvenv.cfg")
	before, _ := os.ReadFile(cfg)
	os.WriteFile(cfg, append(before, []byte("\nchanged = true\n")...), 0600)
	if _, e = s.ResolveKind("one", "python"); e == nil {
		t.Fatal("changed venv metadata accepted")
	}
	os.WriteFile(cfg, before, 0600)
	os.Remove(filepath.Join(venv, "bin", "python"))
	os.Symlink("/bin/true", filepath.Join(venv, "bin", "python"))
	if _, e = s.ResolveKind("one", "python"); e == nil {
		t.Fatal("incompatible venv interpreter accepted")
	}
}
func TestMultiRuntimeApprovalAndConflicts(t *testing.T) {
	path, version := installed(t, "python")
	http := httpFixture(t)
	s := RuntimeService{Store: http.Store}
	st, _ := http.Store.Status()
	root := st.Registry.Projects[0].Root
	req := RuntimeRequest{Action: "select", Name: "one", Kind: "python", Path: path, Version: version}
	p, e := s.Run(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.(state.ProjectPlan).ID
	os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nrequires-python = '>=3.0'\n"), 0600)
	if _, e = s.Run(context.Background(), req); e == nil {
		t.Fatal("metadata change preserved old approval")
	}
	req.Approval = ""
	selectMulti(t, s, req)
	os.WriteFile(filepath.Join(root, ".python-version"), []byte("0.0.0"), 0600)
	if _, e = s.ResolveKind("one", "python"); e == nil {
		t.Fatal("conflicting version file accepted")
	}
	os.Remove(filepath.Join(root, ".python-version"))
	// A newly appeared venv cannot silently change an existing base selection.
	os.Mkdir(filepath.Join(root, ".venv"), 0700)
	if _, e = s.ResolveKind("one", "python"); e == nil {
		t.Fatal("unreviewed venv appearance accepted")
	}
	os.Remove(filepath.Join(root, ".venv"))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if e = s.ExecKind(ctx, "one", "python", []string{"-c", "import time;time.sleep(30)"}, nil, &out, &out); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("cancellation %v", e)
	}
	// Discovery/probing never grants management ownership.
	stateNow, _ := http.Store.Status()
	if len(stateNow.Registry.Resources) != 0 || stateNow.Registry.Projects[1].Python != (domain.RuntimeSelection{}) {
		t.Fatal("selection modified unrelated ownership/project")
	}
}

func TestBunExecDisablesImplicitDependencyInstallation(t *testing.T) {
	path, version := installed(t, "bun")
	httpService := httpFixture(t)
	s := RuntimeService{Store: httpService.Store}
	selectMulti(t, s, RuntimeRequest{Action: "select", Name: "one", Kind: "bun", Path: path, Version: version})
	st, _ := httpService.Store.Status()
	root := st.Registry.Projects[0].Root
	var requests atomic.Int32
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected dependency lookup", 500)
	}))
	defer registry.Close()
	os.WriteFile(filepath.Join(root, ".npmrc"), []byte("registry="+registry.URL+"\n"), 0600)
	os.WriteFile(filepath.Join(root, "bunfig.toml"), []byte("[install]\nauto = 'force'\nregistry = '"+registry.URL+"'\n"), 0600)
	script := filepath.Join(root, "missing.ts")
	os.WriteFile(script, []byte(`import "velnix-nonexistent-no-install-test-package";`), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var out bytes.Buffer
	e := s.ExecKind(ctx, "one", "bun", []string{script}, nil, &out, &out)
	var child *ChildExit
	if !errors.As(e, &child) || child.Code == 0 || requests.Load() != 0 {
		t.Fatalf("implicit install was not refused: %v requests=%d output=%s", e, requests.Load(), &out)
	}
}
