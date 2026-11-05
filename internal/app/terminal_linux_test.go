package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/platform"
	"velnix/internal/state"
)

func TestTerminalReadsPreserveAbsentAndInitializedState(t *testing.T) {
	for _, initialized := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "initialized"}[initialized], func(t *testing.T) {
			root := t.TempDir()
			paths := platform.Paths{Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache")}
			store := state.Store{Paths: paths}
			if initialized {
				plan, e := store.Plan("init")
				if e != nil {
					t.Fatal(e)
				}
				if _, e = store.Apply(context.Background(), "init", plan.ID); e != nil {
					t.Fatal(e)
				}
			}
			snapshot := func() string {
				var b strings.Builder
				filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
					if e != nil {
						t.Fatal(e)
					}
					b.WriteString(p)
					b.WriteString(i.Mode().String())
					if !i.IsDir() {
						data, e := os.ReadFile(p)
						if e != nil {
							t.Fatal(e)
						}
						b.Write(data)
					}
					return nil
				})
				return b.String()
			}
			before := snapshot()
			q := TerminalQuery{Paths: paths, Platform: platform.Detector{Root: "/"}}
			for pass := 0; pass < 2; pass++ {
				for screen := 0; screen < 5; screen++ {
					if len(q.Read(context.Background(), screen)) == 0 {
						t.Fatal("empty screen", screen)
					}
				}
			}
			if after := snapshot(); before != after {
				t.Fatal("terminal query mutated state")
			}
		})
	}
}
func TestTerminalCorruptConfigAndRegistryVisible(t *testing.T) {
	root := t.TempDir()
	paths := platform.Paths{Config: root, Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}
	if e := os.WriteFile(filepath.Join(root, "config.toml"), []byte("[invalid"), 0600); e != nil {
		t.Fatal(e)
	}
	q := TerminalQuery{Paths: paths, Platform: platform.Detector{Root: "/"}}
	entries := q.Read(context.Background(), 4)
	if !strings.Contains(entries[1].Detail, "Unavailable") {
		t.Fatal(entries)
	}
}

func TestTerminalProjectsAndPartialRegistryFailure(t *testing.T) {
	store := httpFixture(t).Store
	q := TerminalQuery{Paths: store.Paths, Platform: platform.Detector{Root: "/"}}
	overview := q.Read(context.Background(), 0)
	foundFPM := false
	for _, e := range overview {
		if e.Name == "FPM one" {
			foundFPM = true
		}
	}
	if !foundFPM {
		t.Fatal("overview missing FPM status")
	}
	projects := q.Read(context.Background(), 1)
	if len(projects) == 0 || projects[0].Name != "one" || !strings.Contains(projects[0].Detail, "Metadata") || !strings.Contains(projects[0].Detail, "Web/FPM profile") {
		t.Fatal(projects)
	}
	if e := os.WriteFile(filepath.Join(store.Paths.Data, "registry.json"), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	projects = q.Read(context.Background(), 1)
	if !strings.Contains(projects[0].Detail, "Unavailable") {
		t.Fatal(projects)
	}
	runtimes := q.Read(context.Background(), 2)
	if len(runtimes) < 6 || runtimes[0].Name != "Provider capabilities" || !strings.Contains(runtimes[len(runtimes)-1].Detail, "Unavailable") {
		t.Fatal(runtimes)
	}
	data, e := os.ReadFile(filepath.Join(store.Paths.Data, "registry.json"))
	if e != nil || string(data) != "corrupt" {
		t.Fatal("corrupt state was changed")
	}
}
