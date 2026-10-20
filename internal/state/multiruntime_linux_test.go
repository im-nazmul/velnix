package state

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velnix/internal/domain"
)

func TestSchemaFourMigrationRetentionAndRecovery(t *testing.T) {
	for _, boundary := range []string{"commit", "before", "after", "external"} {
		t.Run(boundary, func(t *testing.T) {
			s := fixture(t)
			initStore(t, s)
			link := ProjectChange{Action: "link", Name: "demo", Root: t.TempDir(), DocumentRoot: "."}
			p, e := s.PlanProject(link)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.ApplyProject(context.Background(), link, p.ID); e != nil {
				t.Fatal(e)
			}
			php := domain.ExecutableSelection{Path: "/usr/bin/php", Version: "8.3.1", SHA256: strings.Repeat("a", 64), Device: 1, Inode: 2}
			change := ProjectChange{Action: "select-php", Name: "demo", PHPCLI: php}
			p, e = s.PlanProject(change)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.ApplyProject(context.Background(), change, p.ID); e != nil {
				t.Fatal(e)
			}
			selected := domain.RuntimeSelection{Executable: domain.ExecutableSelection{Path: "/opt/node/bin/node", Version: "25.9.0", SHA256: strings.Repeat("b", 64), Device: 1, Inode: 3}}
			change = ProjectChange{Action: "select-runtime", Name: "demo", Kind: "node", Runtime: selected}
			p, e = s.PlanProject(change)
			if e != nil || p.Registry.Schema != 4 || p.Registry.Projects[0].PHPCLI != php {
				t.Fatalf("migration %+v %v", p, e)
			}
			if boundary == "commit" {
				if _, e = s.ApplyProject(context.Background(), change, p.ID); e != nil {
					t.Fatal(e)
				}
				for _, change := range []ProjectChange{{Action: "clear-php", Name: "demo"}, {Action: "proxy", Name: "demo", ProxyPort: 3000}, link} {
					p, e = s.PlanProject(change)
					if e != nil || p.Registry.Schema != 4 || p.Registry.Projects[0].Node != selected {
						t.Fatalf("schema downgraded or runtime lost: %v", e)
					}
					if _, e = s.ApplyProject(context.Background(), change, p.ID); e != nil {
						t.Fatal(e)
					}
				}
				return
			}
			journal := Journal{Schema: Schema, Operation: p.Action, PlanID: p.ID, Status: "applying", BeforeDigest: p.RegistryBefore, TargetDigest: p.TargetDigest}
			os.WriteFile(filepath.Join(s.Paths.State, "journal.json"), encoded(journal), 0600)
			if boundary == "after" {
				os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), encoded(p.Registry), 0600)
			}
			if boundary == "external" {
				b, _ := os.ReadFile(filepath.Join(s.Paths.Data, "registry.json"))
				os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), append(b, ' '), 0600)
			}
			recover, e := s.Plan("recover")
			if boundary == "external" {
				if e == nil {
					t.Fatal("external edit accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			st, e := s.Apply(context.Background(), "recover", recover.ID)
			if e != nil || st.RecoveryRequired {
				t.Fatal(e)
			}
			if (boundary == "after") != (st.Registry.Projects[0].Node == selected) {
				t.Fatal("recovery replayed/lost selection")
			}
		})
	}
}
func TestSchemaThreeRefusesNewSettings(t *testing.T) {
	p, e := domain.ResolveProject("demo", t.TempDir(), ".")
	if e != nil {
		t.Fatal(e)
	}
	p.ProxyPort = 3000
	if e = validateProjects(Registry{Schema: 3, Projects: []domain.Project{p}}); e == nil {
		t.Fatal("old schema accepted proxy")
	}
}
