//go:build linux

package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/platform"
	"velnix/internal/state"
)

func TestDashboardPassiveReadsAndActionAllowlist(t *testing.T) {
	root := t.TempDir()
	paths := platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}
	d := Dashboard{Paths: paths, Platform: platform.Detector{Root: root}}
	for _, page := range []string{"overview", "projects", "runtimes", "services", "certificates", "mail", "diagnostics"} {
		if _, e := d.Read(context.Background(), page); e != nil {
			t.Fatal(page, e)
		}
	}
	files, _ := os.ReadDir(root)
	if len(files) != 0 {
		t.Fatal("read initialized state")
	}
	for _, c := range []DashboardChange{{Operation: "exec"}, {Operation: "start-http"}, {Operation: "initialize", Root: "/secret"}, {Operation: "select-runtime", Kind: "bash"}, {Operation: "link", Name: "test", Root: "relative"}} {
		if _, e := d.Preview(context.Background(), c); e == nil {
			t.Fatal("unsafe action allowed", c)
		}
	}
	init, e := d.Preview(context.Background(), DashboardChange{Operation: "initialize"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Apply(context.Background(), init.Change, init.ID); e != nil {
		t.Fatal(e)
	}
	project := t.TempDir()
	p, e := d.Preview(context.Background(), DashboardChange{Operation: "link", Name: "first", Root: project})
	if e != nil {
		t.Fatal(e)
	}
	other := t.TempDir()
	otherPlan, e := d.Preview(context.Background(), DashboardChange{Operation: "link", Name: "second", Root: other})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Apply(context.Background(), otherPlan.Change, otherPlan.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Apply(context.Background(), p.Change, p.ID); e == nil {
		t.Fatal("stale project plan accepted")
	}
	// UI service DTOs must not leak workload health tokens or provider credentials.
	store := state.Store{Paths: paths}
	e = store.WithHTTP(context.Background(), func(f state.HTTPFiles) error {
		return f.Write("http.json", data(HTTPRecord{Schema: 1, Phase: "stopped", Token: "health-secret"}))
	})
	if e != nil {
		t.Fatal(e)
	}
	v, e := d.Read(context.Background(), "services")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "health-secret") {
		t.Fatal("health token leaked")
	}
}
