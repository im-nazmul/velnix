package state

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestProjectLifecycle(t *testing.T) {
	s := fixture(t)
	change := ProjectChange{Action: "link", Name: "demo", Root: t.TempDir(), DocumentRoot: "."}
	if _, err := s.PlanProject(change); err == nil {
		t.Fatal("link without initialization accepted")
	}
	initStore(t, s)
	before, _ := os.ReadFile(filepath.Join(s.Paths.Data, "registry.json"))
	p, err := s.PlanProject(change)
	if err != nil {
		t.Fatal(err)
	}
	afterPreview, _ := os.ReadFile(filepath.Join(s.Paths.Data, "registry.json"))
	if string(before) != string(afterPreview) {
		t.Fatal("preview changed registry")
	}
	if _, err = s.ApplyProject(context.Background(), change, "bad"); err == nil {
		t.Fatal("bad approval accepted")
	}
	st, err := s.ApplyProject(context.Background(), change, p.ID)
	if err != nil || st.Registry.Schema != 2 || len(st.Registry.Projects) != 1 || st.RecoveryRequired {
		t.Fatalf("%+v %v", st, err)
	}
	p, err = s.PlanProject(change)
	if err != nil || len(p.Effects) != 0 {
		t.Fatalf("link not idempotent: %+v %v", p, err)
	}
	duplicate := change
	duplicate.Name = "duplicate"
	if _, err = s.PlanProject(duplicate); err == nil {
		t.Fatal("duplicate source accepted")
	}
	duplicate = change
	duplicate.Root = t.TempDir()
	if _, err = s.PlanProject(duplicate); err == nil {
		t.Fatal("name conflict accepted")
	}
	// Unlink must work even after the external source directory disappears.
	if err = os.Rename(change.Root, change.Root+"-moved"); err != nil {
		t.Fatal(err)
	}
	change.Action = "unlink"
	p, err = s.PlanProject(change)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ApplyProject(context.Background(), change, p.ID)
	if err != nil || len(st.Registry.Projects) != 0 || st.RecoveryRequired {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err = os.Stat(change.Root + "-moved"); err != nil {
		t.Fatal("unlink touched external directory", err)
	}
}

func TestConcurrentProjectApprovals(t *testing.T) {
	s := fixture(t)
	initStore(t, s)
	changes := []ProjectChange{{Action: "link", Name: "first", Root: t.TempDir(), DocumentRoot: "."}, {Action: "link", Name: "second", Root: t.TempDir(), DocumentRoot: "."}}
	plans := make([]ProjectPlan, 2)
	for i, change := range changes {
		var err error
		plans[i], err = s.PlanProject(change)
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range changes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := s.ApplyProject(context.Background(), changes[i], plans[i].ID)
			errors <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
		}
	}
	st, err := s.Status()
	if err != nil || successes != 1 || len(st.Registry.Projects) != 1 || st.RecoveryRequired {
		t.Fatalf("lost or conflicting concurrent commit: successes=%d status=%+v err=%v", successes, st, err)
	}
}

func TestProjectApprovalBindsDirectoryAndRegistry(t *testing.T) {
	s := fixture(t)
	initStore(t, s)
	change := ProjectChange{Action: "link", Name: "demo", Root: t.TempDir(), DocumentRoot: "."}
	p, err := s.PlanProject(change)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(change.Root, change.Root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(change.Root, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyProject(context.Background(), change, p.ID); err == nil {
		t.Fatal("directory replacement did not invalidate approval")
	}
	p, err = s.PlanProject(change)
	if err != nil {
		t.Fatal(err)
	}
	other := ProjectChange{Action: "link", Name: "other", Root: t.TempDir(), DocumentRoot: "."}
	q, err := s.PlanProject(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyProject(context.Background(), other, q.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyProject(context.Background(), change, p.ID); err == nil {
		t.Fatal("registry change did not invalidate approval")
	}
}

func TestProjectRecoveryBoundaries(t *testing.T) {
	for _, boundary := range []string{"before", "after", "external"} {
		t.Run(boundary, func(t *testing.T) {
			s := fixture(t)
			initStore(t, s)
			change := ProjectChange{Action: "link", Name: "demo", Root: t.TempDir(), DocumentRoot: "."}
			p, err := s.PlanProject(change)
			if err != nil {
				t.Fatal(err)
			}
			j := Journal{Schema: Schema, Operation: p.Action, PlanID: p.ID, Status: "applying", BeforeDigest: p.RegistryBefore, TargetDigest: p.TargetDigest}
			if err = os.WriteFile(filepath.Join(s.Paths.State, "journal.json"), encoded(j), 0600); err != nil {
				t.Fatal(err)
			}
			if boundary == "after" {
				if err = os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), encoded(p.Registry), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if boundary == "external" {
				if err = os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), append(encoded(empty()), ' '), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = s.PlanProject(change); err == nil {
				t.Fatal("mutation accepted during recovery")
			}
			recovery, err := s.Plan("recover")
			if boundary == "external" {
				if err == nil {
					t.Fatal("unrecognized external change accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			st, err := s.Apply(context.Background(), "recover", recovery.ID)
			if err != nil || st.RecoveryRequired {
				t.Fatalf("%+v %v", st, err)
			}
			if (boundary == "after") != (len(st.Registry.Projects) == 1) {
				t.Fatal("recovery replayed or lost registry change")
			}
		})
	}
}
