package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/platform"
	"velnix/internal/state"
)

func TestTerminalManagementReviewApplyAndStalePlan(t *testing.T) {
	root := t.TempDir()
	paths := platform.Paths{Data: filepath.Join(root, "data"), State: filepath.Join(root, "state"), Config: filepath.Join(root, "config")}
	s := NewTerminalSession(context.Background(), TerminalQuery{Paths: paths})
	defer s.Close()
	ctx := context.Background()
	p, e := s.Plan(ctx, TerminalChange{Operation: "initialize"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(paths.Data); !os.IsNotExist(e) {
		t.Fatal("preview created state")
	}
	if e = s.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(root, "source")
	if e = os.Mkdir(source, 0700); e != nil {
		t.Fatal(e)
	}
	link := TerminalChange{Operation: "link", Name: "one", Root: source, DocumentRoot: "."}
	p, e = s.Plan(ctx, link)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(p.Detail, source) {
		t.Fatal("missing reviewed target")
	}
	st, _ := (state.Store{Paths: paths}).Status()
	if len(st.Registry.Projects) != 0 {
		t.Fatal("preview mutated registry")
	}
	other := link
	other.Name = "two"
	other.Root = filepath.Join(root, "other")
	os.Mkdir(other.Root, 0700)
	competing, e := s.Plan(ctx, other)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(ctx, competing); e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(ctx, p); e == nil {
		t.Fatal("stale plan applied")
	}
	p, e = s.Plan(ctx, link)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	unlink, e := s.Plan(ctx, TerminalChange{Operation: "unlink", Name: "one"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(ctx, unlink); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(source); e != nil {
		t.Fatal("unlink removed project source")
	}
	if e = s.Apply(ctx, TerminalPlan{}); e == nil {
		t.Fatal("approval omitted")
	}
	if _, e = s.Plan(ctx, TerminalChange{Operation: "arbitrary-command"}); e == nil {
		t.Fatal("unbounded operation allowed")
	}
}
func TestTerminalRuntimeSelectionAndClear(t *testing.T) {
	path, version := runtimePHP(t)
	store := httpFixture(t).Store
	s := NewTerminalSession(context.Background(), TerminalQuery{Paths: store.Paths})
	defer s.Close()
	p, e := s.Plan(context.Background(), TerminalChange{Operation: "select-runtime", Name: "one", Kind: "php", Executable: path, Version: version})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(p.Detail, "version probe") {
		t.Fatal("probe missing from consent")
	}
	if e = s.Apply(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	if _, e = (RuntimeService{Store: store}).Resolve("one"); e != nil {
		t.Fatal(e)
	}
	clear, e := s.Plan(context.Background(), TerminalChange{Operation: "clear-runtime", Name: "one", Kind: "php"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(context.Background(), clear); e != nil {
		t.Fatal(e)
	}
	if _, e = (RuntimeService{Store: store}).Resolve("one"); e == nil {
		t.Fatal("selection not cleared")
	}
}

func TestTerminalSessionWebLifecycleAndOwnership(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged workload user")
	}
	exe, e := exec.LookPath("nginx")
	if e != nil {
		t.Skip("Nginx unavailable")
	}
	service := httpFixture(t)
	ctx := context.Background()
	session := NewTerminalSession(ctx, TerminalQuery{Paths: service.Store.Paths})
	defer session.Close()
	change := TerminalChange{Operation: "start-http", Executable: exe, Port: freePort(t)}
	apply := func(c TerminalChange) {
		t.Helper()
		p, e := session.Plan(ctx, c)
		if e != nil {
			t.Fatal(e)
		}
		if e = session.Apply(ctx, p); e != nil {
			t.Fatal(e)
		}
	}
	apply(change)
	if code, _, e := requestHTTP(change.Port, "one.test", "/"); e != nil || code != 200 {
		t.Fatal("session web not serving", code, e)
	}
	apply(TerminalChange{Operation: "reload-http"})
	apply(TerminalChange{Operation: "stop-http"})
	// A successful stop must finish session-owner cleanup before an immediate restart.
	apply(change)
	if e = session.Close(); e != nil {
		t.Fatal(e)
	}
	st, e := service.Status()
	if e != nil || st.Alive || st.Record.Phase != "stopped" {
		t.Fatal("session exit left workload", st, e)
	}
	p, e := session.Plan(ctx, TerminalChange{Operation: "stop-http"})
	if e != nil {
		t.Fatal(e)
	}
	if e = session.Apply(ctx, p); !errors.Is(e, context.Canceled) {
		t.Fatal("closed session accepted apply", e)
	}
	// A fresh session observing a separate foreground owner cannot adopt or stop
	// that owner simply by closing.
	port := freePort(t)
	startPool(t, service, HTTPRequest{Action: "start", Executable: exe, Port: port})
	observer := NewTerminalSession(ctx, TerminalQuery{Paths: service.Store.Paths})
	observer.Read(ctx, 3)
	if e = observer.Close(); e != nil {
		t.Fatal(e)
	}
	st, e = service.Status()
	if e != nil || !st.Alive {
		t.Fatal("observer stopped another owner", st, e)
	}
}
func TestTerminalSessionFPMLifetime(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged workload user")
	}
	exe := "/usr/sbin/php-fpm8.3"
	if _, e := os.Stat(exe); e != nil {
		t.Skip("PHP-FPM unavailable")
	}
	banner, e := exec.Command(exe, "-n", "-v").Output()
	if e != nil {
		t.Fatal(e)
	}
	version := strings.Fields(string(banner))[1]
	service := httpFixture(t)
	session := NewTerminalSession(context.Background(), TerminalQuery{Paths: service.Store.Paths})
	defer session.Close()
	p, e := session.Plan(context.Background(), TerminalChange{Operation: "start-fpm", Name: "one", Executable: exe, Version: version, Profile: "php", Uploads: "uploads"})
	if e != nil {
		t.Fatal(e)
	}
	if e = session.Apply(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	pool := HTTPService{Store: service.Store, FPMName: "one"}
	st, e := pool.Status()
	if e != nil || !st.Alive {
		t.Fatal(st, e)
	}
	if e = session.Close(); e != nil {
		t.Fatal(e)
	}
	st, e = pool.Status()
	if e != nil || st.Alive {
		t.Fatal("FPM survived session exit", st, e)
	}
}
