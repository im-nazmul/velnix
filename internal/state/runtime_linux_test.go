package state

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/domain"
)

func TestPHPSelectionMigrationAndRecovery(t *testing.T) {
	for _, boundary := range []string{"commit", "before", "after", "external"} {
		t.Run(boundary, func(t *testing.T) {
			s := fixture(t)
			initStore(t, s)
			link := ProjectChange{Action: "link", Name: "demo", Root: t.TempDir(), DocumentRoot: "."}
			p, err := s.PlanProject(link)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ApplyProject(context.Background(), link, p.ID); err != nil {
				t.Fatal(err)
			}
			selection := domain.PHPSelection{Path: "/usr/bin/php8.3", Version: "8.3.12", SHA256: strings.Repeat("a", 64), Device: 1, Inode: 2}
			change := ProjectChange{Action: "select-php", Name: "demo", PHPCLI: selection}
			p, err = s.PlanProject(change)
			if err != nil || p.Registry.Schema != 3 || p.Registry.Projects[0].PHPCLI != selection {
				t.Fatalf("%+v %v", p, err)
			}
			if boundary == "commit" {
				st, err := s.ApplyProject(context.Background(), change, p.ID)
				if err != nil || st.Registry.Schema != 3 || st.RecoveryRequired {
					t.Fatalf("%+v %v", st, err)
				}
				other := ProjectChange{Action: "link", Name: "other", Root: t.TempDir(), DocumentRoot: "."}
				p, err = s.PlanProject(other)
				if err != nil || p.Registry.Schema != 3 {
					t.Fatalf("schema downgraded: %+v %v", p, err)
				}
				if _, err = s.ApplyProject(context.Background(), other, p.ID); err != nil {
					t.Fatal(err)
				}
				st, err = s.Status()
				if err != nil || st.Registry.Projects[0].PHPCLI != selection {
					t.Fatal("selection lost", err)
				}
				return
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
				b, err := os.ReadFile(filepath.Join(s.Paths.Data, "registry.json"))
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), append(b, ' '), 0600); err != nil {
					t.Fatal(err)
				}
			}
			recover, err := s.Plan("recover")
			if boundary == "external" {
				if err == nil {
					t.Fatal("external edit accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			st, err := s.Apply(context.Background(), "recover", recover.ID)
			if err != nil || st.RecoveryRequired {
				t.Fatalf("%+v %v", st, err)
			}
			if (boundary == "after") != (st.Registry.Projects[0].PHPCLI == selection) {
				t.Fatal("recovery replayed or lost selection")
			}
		})
	}
}

func TestPHPSelectionSchemaValidation(t *testing.T) {
	p, err := domain.ResolveProject("demo", t.TempDir(), ".")
	if err != nil {
		t.Fatal(err)
	}
	p.PHPCLI = domain.PHPSelection{Path: "/usr/bin/php8.3", Version: "8.3.12", SHA256: strings.Repeat("a", 64), Device: 1, Inode: 2}
	if err = validateProjects(Registry{Schema: 2, Projects: []domain.Project{p}}); err == nil {
		t.Fatal("schema 2 accepted runtime fields")
	}
	p.PHPCLI.Version = "8.3"
	if err = validateProjects(Registry{Schema: 3, Projects: []domain.Project{p}}); err == nil {
		t.Fatal("partial version accepted")
	}
}
