//go:build linux

package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"velnix/internal/domain"
	"velnix/internal/platform"
)

const Schema = 1

// Resource grants remain explicit; discovering a resource never creates a record here.
type Resource struct {
	ID         string   `json:"id"`
	Provider   string   `json:"provider"`
	Origin     string   `json:"origin"`
	Authority  string   `json:"authority"`
	Grants     []string `json:"grants"`
	References []string `json:"references"`
}
type Registry struct {
	Schema    int              `json:"schema"`
	Resources []Resource       `json:"resources"`
	Projects  []domain.Project `json:"projects,omitempty"`
}
type Journal struct {
	Schema       int       `json:"schema"`
	Operation    string    `json:"operation"`
	PlanID       string    `json:"plan_id"`
	Status       string    `json:"status"`
	TargetDigest string    `json:"target_digest"`
	BeforeDigest string    `json:"before_digest,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Status struct {
	Initialized      bool     `json:"initialized"`
	Registry         Registry `json:"registry"`
	Journal          *Journal `json:"journal,omitempty"`
	RecoveryRequired bool     `json:"recovery_required"`
}
type Plan struct {
	Schema         int      `json:"schema"`
	Action         string   `json:"action"`
	DataPath       string   `json:"data_path"`
	StatePath      string   `json:"state_path"`
	RegistryBefore string   `json:"registry_before"`
	JournalBefore  string   `json:"journal_before"`
	TargetDigest   string   `json:"target_digest"`
	Effects        []string `json:"effects"`
	Privilege      string   `json:"privilege"`
	Network        bool     `json:"network"`
	Limitations    string   `json:"limitations"`
	ID             string   `json:"id"`
}
type Store struct{ Paths platform.Paths }

func empty() Registry      { return Registry{Schema: Schema, Resources: []Resource{}} }
func encoded(v any) []byte { b, _ := json.MarshalIndent(v, "", "  "); return append(b, '\n') }
func digest(b []byte) string {
	if b == nil {
		return "absent"
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func decode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return domain.E(domain.Invalid, "invalid state JSON: %v", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return domain.E(domain.Invalid, "state JSON contains trailing data")
	}
	return nil
}
func (s Store) snapshots() ([]byte, []byte, error) {
	r, err := readAt(s.Paths.Data, "registry.json")
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return nil, nil, ioError(err)
	}
	j, err := readAt(s.Paths.State, "journal.json")
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return r, j, ioError(err)
}
func parse(r, j []byte) (Status, error) {
	st := Status{Registry: empty()}
	if r != nil {
		st.Registry = Registry{}
		if err := decode(r, &st.Registry); err != nil {
			return st, err
		}
		if st.Registry.Schema != Schema && st.Registry.Schema != 2 {
			return st, domain.E(domain.Invalid, "unsupported registry schema %d", st.Registry.Schema)
		}
		if st.Registry.Resources == nil {
			return st, domain.E(domain.Invalid, "registry resources must be an array")
		}
		if err := validateProjects(st.Registry); err != nil {
			return st, err
		}
		st.Initialized = true
	}
	if j != nil {
		st.Journal = &Journal{}
		if err := decode(j, st.Journal); err != nil {
			return st, err
		}
		projectOperation := st.Journal.Operation == "project-link" || st.Journal.Operation == "project-unlink"
		if st.Journal.Schema != Schema || (st.Journal.Operation != "init" && !projectOperation) || !validDigest(st.Journal.PlanID) || !validDigest(st.Journal.TargetDigest) || (projectOperation && !validDigest(st.Journal.BeforeDigest)) || (st.Journal.Operation == "init" && st.Journal.TargetDigest != digest(encoded(empty()))) {
			return st, domain.E(domain.Invalid, "unsupported or invalid operation journal")
		}
		switch st.Journal.Status {
		case "planned", "approved", "applying", "verified", "recovery-required":
			st.RecoveryRequired = true
		case "complete":
			if !st.Initialized || digest(r) != st.Journal.TargetDigest {
				st.RecoveryRequired = true
			}
		case "failed":
			st.RecoveryRequired = st.Initialized
			if projectOperation {
				st.RecoveryRequired = digest(r) != st.Journal.BeforeDigest
			}
		default:
			return st, domain.E(domain.Invalid, "invalid journal status")
		}
	}
	if r != nil && j == nil {
		return st, domain.E(domain.Conflict, "registry exists without initialization evidence; preserve it for manual review")
	}
	return st, nil
}
func (s Store) Status() (Status, error) {
	r, j, err := s.snapshots()
	if err != nil {
		return Status{}, err
	}
	return parse(r, j)
}
func (s Store) Plan(action string) (Plan, error) {
	r, j, err := s.snapshots()
	if err != nil {
		return Plan{}, err
	}
	st, err := parse(r, j)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{Schema: Schema, Action: action, DataPath: s.Paths.Data, StatePath: s.Paths.State, RegistryBefore: digest(r), JournalBefore: digest(j), TargetDigest: digest(encoded(empty())), Effects: []string{}, Privilege: "current-user", Limitations: "Private directories and lock files may remain after failure; existing state is never deleted automatically."}
	switch action {
	case "init":
		if st.RecoveryRequired {
			return p, domain.E(domain.Conflict, "interrupted operation requires state recover")
		}
		if !st.Initialized {
			p.Effects = []string{"create missing private data/state directories and mutation lock", "journal approved initialization", "atomically create empty schema-1 registry", "mark initialization verified and complete"}
		}
	case "recover":
		if st.RecoveryRequired {
			if st.Journal.Operation != "init" {
				p.TargetDigest = st.Journal.TargetDigest
				if digest(r) == st.Journal.TargetDigest {
					p.Effects = []string{"mark already committed project registry change complete"}
				} else if digest(r) == st.Journal.BeforeDigest {
					p.Effects = []string{"mark uncommitted project registry change failed; do not replay"}
				} else {
					return p, domain.E(domain.Conflict, "registry differs from recorded before and target digests; manual review required")
				}
				break
			}
			if r != nil && digest(r) != st.Journal.TargetDigest {
				return p, domain.E(domain.Conflict, "registry differs from initialization target; manual review required")
			}
			if r == nil {
				p.Effects = []string{"mark interrupted initialization failed; leave directories intact"}
			} else {
				p.Effects = []string{"mark already committed initialization complete"}
			}
		}
	default:
		return p, domain.E(domain.Invalid, "unsupported state action %q", action)
	}
	p.ID = digest(encoded(p))
	return p, nil
}
func (s Store) Apply(ctx context.Context, action, approvedID string) (Status, error) {
	p, err := s.Plan(action)
	if err != nil {
		return Status{}, err
	}
	if p.ID != approvedID {
		return Status{}, domain.E(domain.Conflict, "plan changed or approval digest does not match; preview again")
	}
	if len(p.Effects) == 0 {
		return s.Status()
	}
	if err = ctx.Err(); err != nil {
		return Status{}, err
	}
	state, err := openDir(s.Paths.State, true)
	if err != nil {
		return Status{}, ioError(err)
	}
	defer state.close()
	lock, err := state.open("mutation.lock", syscall.O_RDWR|syscall.O_CREAT)
	if err != nil {
		return Status{}, ioError(err)
	}
	defer lock.Close()
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return Status{}, err
		}
		select {
		case <-ctx.Done():
			return Status{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	current, err := s.Plan(action)
	if err != nil {
		return Status{}, err
	}
	if current.ID != p.ID {
		return Status{}, domain.E(domain.Conflict, "state changed while acquiring lock; preview again")
	}
	if action == "recover" {
		b, err := state.read("journal.json")
		if err != nil {
			return Status{}, err
		}
		var j Journal
		if err = decode(b, &j); err != nil {
			return Status{}, err
		}
		j.Status = "complete"
		if p.RegistryBefore == "absent" || (j.Operation != "init" && p.RegistryBefore == j.BeforeDigest) {
			j.Status = "failed"
		}
		j.UpdatedAt = time.Now().UTC()
		if err = state.write("journal.json", encoded(j)); err != nil {
			return Status{}, ioError(err)
		}
		return s.Status()
	}
	j := Journal{Schema: Schema, Operation: "init", PlanID: p.ID, TargetDigest: p.TargetDigest}
	record := func(status string) error {
		j.Status = status
		j.UpdatedAt = time.Now().UTC()
		return state.write("journal.json", encoded(j))
	}
	if err = record("approved"); err != nil {
		return Status{}, ioError(err)
	}
	if err = record("applying"); err != nil {
		return Status{}, ioError(err)
	}
	data, err := openDir(s.Paths.Data, true)
	if err != nil {
		return Status{}, ioError(err)
	}
	defer data.close()
	if err = ctx.Err(); err != nil {
		return Status{}, err
	}
	// Recheck destination after journaling; never overwrite a concurrently introduced registry.
	if _, e := data.read("registry.json"); !errors.Is(e, os.ErrNotExist) {
		return Status{}, domain.E(domain.Conflict, "registry appeared during initialization; manual review required")
	}
	if err = data.publish("registry.json", encoded(empty()), true); err != nil {
		return Status{}, ioError(err)
	}
	b, err := data.read("registry.json")
	if err != nil {
		return Status{}, err
	}
	if digest(b) != p.TargetDigest {
		return Status{}, domain.E(domain.Conflict, "registry verification failed")
	}
	if err = record("verified"); err != nil {
		return Status{}, ioError(err)
	}
	if err = record("complete"); err != nil {
		return Status{}, ioError(err)
	}
	return s.Status()
}
