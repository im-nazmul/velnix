//go:build linux

package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"velnix/internal/platform"
	"velnix/internal/process"
	"velnix/internal/state"
)

func httpFixture(t *testing.T) HTTPService {
	t.Helper()
	base := t.TempDir()
	s := state.Store{Paths: platform.Paths{Data: filepath.Join(base, "data"), State: filepath.Join(base, "state")}}
	p, e := s.Plan("init")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), "init", p.ID); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"one", "two"} {
		root := filepath.Join(base, name)
		if e = os.Mkdir(root, 0700); e != nil {
			t.Fatal(e)
		}
		for file, body := range map[string]string{"index.html": name, ".env": "secret", "index.php": "secret", "hello.txt": "hello"} {
			if e = os.WriteFile(filepath.Join(root, file), []byte(body), 0600); e != nil {
				t.Fatal(e)
			}
		}
		if e = os.Symlink("/etc/passwd", filepath.Join(root, "escape.txt")); e != nil {
			t.Fatal(e)
		}
		change := state.ProjectChange{Action: "link", Name: name, Root: root, DocumentRoot: "."}
		plan, e := s.PlanProject(change)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.ApplyProject(context.Background(), change, plan.ID); e != nil {
			t.Fatal(e)
		}
	}
	return HTTPService{Store: s}
}
func requestHTTP(port int, host, path string) (int, string, error) {
	client := http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	req.Host = host
	r, e := client.Do(req)
	if e != nil {
		return 0, "", e
	}
	defer r.Body.Close()
	b, e := io.ReadAll(r.Body)
	return r.StatusCode, string(b), e
}
func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}
func waitRunning(t *testing.T, s HTTPService) {
	t.Helper()
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		st, e := s.Status()
		if e == nil && st.Record != nil && st.Record.Phase == "running" && st.Alive {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, e := s.Status()
	t.Fatalf("not running: %+v %v", st, e)
}
func TestHTTPRealLifecycle(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged workload user")
	}
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("installed Nginx required")
	}
	s := httpFixture(t)
	externalBefore := externalNginxSnapshot(t)
	t.Cleanup(func() {
		after := externalNginxSnapshot(t)
		if string(data(after)) != string(data(externalBefore)) {
			t.Error("external Nginx configuration changed")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := HTTPRequest{Action: "start", Executable: exe, Port: freePort(t)}
	p, e := s.Plan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Store.Paths.State, "http.conf")); !os.IsNotExist(e) {
		t.Fatal("preview wrote config")
	}
	req.Approval = p.ID
	done := make(chan error, 1)
	go func() { _, e := s.Apply(ctx, req); done <- e }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(8 * time.Second):
			t.Error("foreground did not exit")
		}
	})
	waitRunning(t, s)
	for _, tt := range []struct {
		host, path, body string
		code             int
	}{{"one.test", "/", "one", 200}, {"two.test", "/", "two", 200}, {"one.test", "/hello.txt", "hello", 200}, {"unknown.test", "/", "", 404}, {"one.test", "/.env", "", 404}, {"one.test", "/index.php", "", 404}, {"one.test", "/escape.txt", "", 404}} {
		code, body, e := requestHTTP(req.Port, tt.host, tt.path)
		if e != nil || code != tt.code || (tt.body != "" && body != tt.body) {
			t.Fatalf("%s%s: %d %q %v", tt.host, tt.path, code, body, e)
		}
	}
	// A duplicate apply must not launch another workload.
	if _, e = s.Apply(context.Background(), req); e == nil {
		t.Fatal("duplicate startup approval was accepted")
	}
	// Registry edits do not silently mutate the active routing snapshot.
	change := state.ProjectChange{Action: "unlink", Name: "two"}
	unlink, e := s.Store.PlanProject(change)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.ApplyProject(ctx, change, unlink.ID); e != nil {
		t.Fatal(e)
	}
	st, e := s.Status()
	if e != nil || !st.PendingRouting {
		t.Fatalf("missing pending route: %+v %v", st, e)
	}
	reload := HTTPRequest{Action: "reload"}
	rp, e := s.Plan(ctx, reload)
	if e != nil {
		t.Fatal(e)
	}
	reload.Approval = rp.ID
	if _, e = s.Apply(ctx, reload); e != nil {
		t.Fatal(e)
	}
	code, _, e := requestHTTP(req.Port, "two.test", "/")
	if e != nil || code != 404 {
		t.Fatalf("unlinked route still served: %d %v", code, e)
	}
	// Stale approvals are rejected and externally modified config is preserved.
	stop := HTTPRequest{Action: "stop"}
	sp, e := s.Plan(ctx, stop)
	if e != nil {
		t.Fatal(e)
	}
	reload.Approval = "wrong"
	if _, e = s.Apply(ctx, reload); e == nil {
		t.Fatal("accepted wrong approval")
	}
	stop.Approval = sp.ID
	if _, e = s.Apply(ctx, stop); e != nil {
		t.Fatal(e)
	}
	st, e = s.Status()
	if e != nil || st.Alive || st.Record.Phase != "stopped" {
		t.Fatalf("not stopped: %+v %v", st, e)
	}
}
func TestHTTPOccupiedPortAndRecovery(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged user")
	}
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("Nginx not installed")
	}
	s := httpFixture(t)
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	req := HTTPRequest{Action: "start", Executable: exe, Port: l.Addr().(*net.TCPAddr).Port}
	p, e := s.Plan(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	if _, e = s.Apply(context.Background(), req); e == nil || !strings.Contains(e.Error(), "occupied") {
		t.Fatalf("occupied: %v", e)
	}
	if b, _ := s.Store.ReadHTTP(); b != nil {
		t.Fatal("occupied port changed lifecycle evidence")
	}
	// Known dead identity can be reconciled without launching anything.
	r := HTTPRecord{Schema: 1, Phase: "running", Port: req.Port}
	if e = s.Store.WithHTTP(context.Background(), func(f state.HTTPFiles) error { return f.Write("http.json", data(r)) }); e != nil {
		t.Fatal(e)
	}
	recover := HTTPRequest{Action: "recover"}
	rp, e := s.Plan(context.Background(), recover)
	if e != nil {
		t.Fatal(e)
	}
	recover.Approval = rp.ID
	if _, e = s.Apply(context.Background(), recover); e != nil {
		t.Fatal(e)
	}
	st, e := s.Status()
	if e != nil || st.Alive || st.RecoveryRequired {
		t.Fatalf("bad recovery %+v %v", st, e)
	}
}

func TestHTTPCancellationAndExternalEdits(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged user")
	}
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("installed Nginx required")
	}
	s := httpFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := HTTPRequest{Action: "start", Executable: exe, Port: freePort(t)}
	p, e := s.Plan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	done := make(chan error, 1)
	go func() { _, e := s.Apply(ctx, req); done <- e }()
	waitRunning(t, s)
	before, _ := s.Status()
	cancel()
	select {
	case e = <-done:
		if e != context.Canceled {
			t.Fatalf("cancel: %v", e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("cancellation left foreground alive")
	}
	st, e := s.Status()
	if e != nil || st.Alive || st.RecoveryRequired {
		t.Fatalf("cancel status %+v %v", st, e)
	}
	if before.Record == nil {
		t.Fatal("missing record")
	}
	changed := []byte("# external edit\n")
	if e = os.WriteFile(filepath.Join(s.Store.Paths.State, "http.conf"), changed, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Plan(context.Background(), req); e == nil {
		t.Fatal("accepted externally modified config")
	}
	actual, _ := os.ReadFile(filepath.Join(s.Store.Paths.State, "http.conf"))
	if string(actual) != string(changed) {
		t.Fatal("external edit overwritten")
	}
}

func TestHTTPKilledMasterCleansRecordedWorkers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged user")
	}
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("installed Nginx required")
	}
	s := httpFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := HTTPRequest{Action: "start", Executable: exe, Port: freePort(t)}
	p, e := s.Plan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	done := make(chan error, 1)
	go func() { _, e := s.Apply(ctx, req); done <- e }()
	waitRunning(t, s)
	st, e := s.Status()
	if e != nil {
		t.Fatal(e)
	}
	if len(st.Record.Workers) == 0 {
		t.Fatal("missing worker evidence")
	}
	if e = process.Signal(st.Record.Process, unix.SIGKILL); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		if e == nil {
			t.Fatal("unexpected exit reported success")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("master exit left foreground running")
	}
	for _, worker := range st.Record.Workers {
		if process.Alive(worker) {
			t.Fatal("orphaned owned worker")
		}
	}
	st, e = s.Status()
	if e != nil || !st.RecoveryRequired {
		t.Fatalf("missing recovery state %+v %v", st, e)
	}
	recover := HTTPRequest{Action: "recover"}
	rp, e := s.Plan(context.Background(), recover)
	if e != nil {
		t.Fatal(e)
	}
	recover.Approval = rp.ID
	if _, e = s.Apply(context.Background(), recover); e != nil {
		t.Fatal(e)
	}
}

func externalNginxSnapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	e := filepath.WalkDir("/etc/nginx", func(path string, entry os.DirEntry, e error) error {
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if entry.Type().IsRegular() {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			out[path] = hash(b)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
