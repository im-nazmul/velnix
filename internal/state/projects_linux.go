//go:build linux

package state

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"syscall"
	"time"

	"velnix/internal/domain"
)

type ProjectChange struct {
	Action       string
	Name         string
	Root         string
	DocumentRoot string
	PHPCLI       domain.PHPSelection
}

type ProjectPlan struct {
	Plan
	Registry Registry `json:"registry_target"`
}

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32
}

func validateProjects(r Registry) error {
	if r.Schema == 1 && len(r.Projects) != 0 {
		return domain.E(domain.Invalid, "projects require registry schema 2")
	}
	if len(r.Projects) > 1000 {
		return domain.E(domain.Invalid, "project registry limit is 1000 entries")
	}
	names, roots := map[string]bool{}, map[string]bool{}
	for _, p := range r.Projects {
		if r.Schema < 3 && p.PHPCLI != (domain.PHPSelection{}) {
			return domain.E(domain.Invalid, "PHP selections require registry schema 3")
		}
		if err := p.Validate(); err != nil {
			return err
		}
		if names[p.Name] || roots[p.Root] {
			return domain.E(domain.Invalid, "duplicate project name or root in registry")
		}
		names[p.Name], roots[p.Root] = true, true
	}
	return nil
}

func (s Store) PlanProject(change ProjectChange) (ProjectPlan, error) {
	var p ProjectPlan
	if change.Action != "link" && change.Action != "unlink" && change.Action != "select-php" && change.Action != "clear-php" {
		return p, domain.E(domain.Invalid, "unknown project mutation")
	}
	if !domain.ValidProjectName(change.Name) {
		return p, domain.E(domain.Invalid, "invalid project name")
	}
	r, j, err := s.snapshots()
	if err != nil {
		return p, err
	}
	st, err := parse(r, j)
	if err != nil {
		return p, err
	}
	if !st.Initialized {
		return p, domain.E(domain.Unavailable, "initialize private state with state init before linking projects")
	}
	if st.RecoveryRequired {
		return p, domain.E(domain.Conflict, "interrupted operation requires state recover")
	}
	target := st.Registry
	if target.Schema < 2 {
		target.Schema = 2
	}
	p.Plan = Plan{Schema: Schema, Action: "project-" + change.Action, DataPath: s.Paths.Data, StatePath: s.Paths.State, RegistryBefore: digest(r), JournalBefore: digest(j), Effects: []string{}, Privilege: "current-user", Limitations: "Only registry metadata changes. Project files remain external. Recovery recognizes exact before/after digests; unknown edits require manual review. Older Velnix binaries cannot read schema 2."}
	index := -1
	for i, project := range target.Projects {
		if project.Name == change.Name {
			index = i
		}
	}
	switch change.Action {
	case "link":
		project, e := domain.ResolveProject(change.Name, change.Root, change.DocumentRoot)
		if e != nil {
			return p, e
		}
		if index >= 0 {
			project.PHPCLI = target.Projects[index].PHPCLI
			if target.Projects[index] != project {
				return p, domain.E(domain.Conflict, "project name already linked with different metadata; unlink it first")
			}
		} else {
			for _, existing := range target.Projects {
				if existing.Root == project.Root || (existing.Device == project.Device && existing.Inode == project.Inode) {
					return p, domain.E(domain.Conflict, "project directory is already linked as %s", existing.Name)
				}
			}
			target.Projects = append(target.Projects, project)
			p.Effects = []string{"journal approved project link", "atomically replace registry with reviewed project metadata", "verify registry and mark operation complete"}
		}
	case "select-php", "clear-php":
		if index < 0 {
			return p, domain.E(domain.Unavailable, "project is not linked")
		}
		selection := change.PHPCLI
		if change.Action == "select-php" {
			if err := selection.Validate(); err != nil {
				return p, err
			}
		} else {
			selection = domain.PHPSelection{}
		}
		if target.Projects[index].PHPCLI != selection {
			target.Schema = 3
			target.Projects[index].PHPCLI = selection
			p.Effects = []string{"journal approved PHP CLI selection change", "atomically replace registry with exact external PHP CLI selection; preserve all other projects and resources", "verify registry and mark operation complete"}
			if change.Action == "select-php" {
				p.Effects = append([]string{`execute selected PHP with argv [-n, -r, echo json_encode(["version"=>PHP_VERSION,"sapi"=>PHP_SAPI]);], cwd /, environment PATH=/usr/bin:/bin LANG=C; timeout 2s; require exact version and cli SAPI before registry commit`}, p.Effects...)
			}
			p.Limitations = "Registry metadata only; external runtime remains externally owned. Selection approval also authorizes a bounded PHP CLI probe before commit. Existing binaries cannot read schema 3. Clearing selection does not stop already running commands."
		}
	case "unlink":
		if index >= 0 {
			target.Projects = append(target.Projects[:index:index], target.Projects[index+1:]...)
			p.Effects = []string{"journal approved project unlink", "atomically remove registry entry; preserve project source and data", "verify registry and mark operation complete"}
		}
	}
	sort.Slice(target.Projects, func(i, j int) bool { return target.Projects[i].Name < target.Projects[j].Name })
	if err = validateProjects(target); err != nil {
		return p, err
	}
	if len(p.Effects) == 0 {
		target = st.Registry
	}
	if len(p.Effects) > 0 && st.Registry.Schema == 1 {
		p.Effects = append([]string{"upgrade registry from schema 1 to schema 2; retain existing resources"}, p.Effects...)
	}
	if len(p.Effects) > 0 && target.Schema == 3 && st.Registry.Schema < 3 {
		p.Effects = append([]string{"upgrade registry to schema 3; retain existing projects and resources"}, p.Effects...)
	}
	p.Registry = target
	if len(encoded(target)) > maxFile {
		return p, domain.E(domain.Invalid, "registry exceeds size limit")
	}
	p.TargetDigest = digest(encoded(target))
	p.ID = digest(encoded(p))
	return p, nil
}

func (s Store) ApplyProject(ctx context.Context, change ProjectChange, approvedID string) (Status, error) {
	p, err := s.PlanProject(change)
	if err != nil {
		return Status{}, err
	}
	if p.ID != approvedID {
		return Status{}, domain.E(domain.Conflict, "plan changed or approval digest does not match; preview again")
	}
	if err = ctx.Err(); err != nil {
		return Status{}, err
	}
	if len(p.Effects) == 0 {
		return s.Status()
	}
	d, err := openDir(s.Paths.State, false)
	if err != nil {
		return Status{}, ioError(err)
	}
	defer d.close()
	lock, err := d.open("mutation.lock", syscall.O_RDWR|syscall.O_CREAT)
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
	current, err := s.PlanProject(change)
	if err != nil {
		return Status{}, err
	}
	if current.ID != p.ID {
		return Status{}, domain.E(domain.Conflict, "state or project changed while acquiring lock; preview again")
	}
	j := Journal{Schema: Schema, Operation: p.Action, PlanID: p.ID, BeforeDigest: p.RegistryBefore, TargetDigest: p.TargetDigest}
	record := func(status string) error {
		j.Status = status
		j.UpdatedAt = time.Now().UTC()
		return ioError(d.write("journal.json", encoded(j)))
	}
	if err = record("approved"); err != nil {
		return Status{}, err
	}
	if err = record("applying"); err != nil {
		return Status{}, err
	}
	data, err := openDir(s.Paths.Data, false)
	if err != nil {
		return Status{}, ioError(err)
	}
	defer data.close()
	before, err := data.read("registry.json")
	if err != nil {
		return Status{}, ioError(err)
	}
	if digest(before) != p.RegistryBefore {
		return Status{}, domain.E(domain.Conflict, "registry changed after approval; preserve evidence and run state recover")
	}
	if err = ctx.Err(); err != nil {
		return Status{}, err
	}
	if len(encoded(p.Registry)) > maxFile {
		return Status{}, domain.E(domain.Invalid, "registry exceeds size limit")
	}
	if err = data.write("registry.json", encoded(p.Registry)); err != nil {
		return Status{}, ioError(err)
	}
	after, err := data.read("registry.json")
	if err != nil {
		return Status{}, ioError(err)
	}
	if digest(after) != p.TargetDigest {
		return Status{}, domain.E(domain.Conflict, "registry verification failed; run state recover")
	}
	if err = record("verified"); err != nil {
		return Status{}, err
	}
	if err = record("complete"); err != nil {
		return Status{}, err
	}
	return s.Status()
}
