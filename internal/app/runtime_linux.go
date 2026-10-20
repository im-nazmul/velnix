//go:build linux

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"velnix/internal/domain"
	"velnix/internal/process"
	"velnix/internal/providers/phpcli"
	"velnix/internal/state"
)

type RuntimeRequest struct {
	Action, Name, Path, Version, Approval string
	DryRun                                bool
}

type ProbePlan struct {
	Action      string              `json:"action"`
	Executable  domain.PHPSelection `json:"executable"`
	Argv        []string            `json:"argv"`
	Directory   string              `json:"directory"`
	Environment []string            `json:"environment"`
	Limitations string              `json:"limitations"`
	ID          string              `json:"id"`
}

type ExecutionSpec struct {
	Executable  domain.PHPSelection `json:"executable"`
	Directory   string              `json:"directory"`
	Environment map[string]string   `json:"environment_delta"`
}

type RuntimeService struct{ Store ProjectStore }

func (s RuntimeService) Run(ctx context.Context, req RuntimeRequest) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.DryRun && req.Approval != "" {
		return nil, domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
	}
	switch req.Action {
	case "list":
		return phpcli.Discover(), nil
	case "inspect":
		selection, err := phpcli.Snapshot(req.Path, req.Version)
		if err != nil {
			return nil, err
		}
		p := ProbePlan{Action: "probe-php-cli", Executable: selection, Argv: []string{"-n", "-r", `echo json_encode(["version"=>PHP_VERSION,"sapi"=>PHP_SAPI]);`}, Directory: "/", Environment: []string{"PATH=/usr/bin:/bin", "LANG=C"}, Limitations: "Runs the explicitly selected external executable with no php.ini, project files or inherited environment. No installation, ownership or persistent state changes. Probe timeout: 2 seconds; retained stdout/stderr: 4 KiB each."}
		b, _ := json.Marshal(p)
		h := sha256.Sum256(b)
		p.ID = hex.EncodeToString(h[:])
		if req.Approval == "" {
			return p, nil
		}
		if req.Approval != p.ID {
			return nil, domain.E(domain.Conflict, "PHP probe plan changed; preview again")
		}
		if err = phpcli.Probe(ctx, selection); err != nil {
			return nil, err
		}
		return selection, nil
	case "select", "clear":
		change := state.ProjectChange{Action: "clear-php", Name: req.Name}
		if req.Action == "select" {
			selection, err := phpcli.Snapshot(req.Path, req.Version)
			if err != nil {
				return nil, err
			}
			change.Action, change.PHPCLI = "select-php", selection
		}
		p, err := s.Store.PlanProject(change)
		if err != nil {
			return nil, err
		}
		if req.Approval == "" {
			return p, nil
		}
		if req.Approval != p.ID {
			return nil, domain.E(domain.Conflict, "PHP selection plan changed; preview again")
		}
		if req.Action == "select" && len(p.Effects) != 0 {
			if err = phpcli.Probe(ctx, change.PHPCLI); err != nil {
				return nil, err
			}
		}
		applyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return s.Store.ApplyProject(applyCtx, change, req.Approval)
	case "resolve":
		return s.Resolve(req.Name)
	default:
		return nil, domain.E(domain.Invalid, "unknown runtime subcommand %q", req.Action)
	}
}

func (s RuntimeService) project(name string) (domain.Project, error) {
	if !domain.ValidProjectName(name) {
		return domain.Project{}, domain.E(domain.Invalid, "invalid project name")
	}
	st, err := s.Store.Status()
	if err != nil {
		return domain.Project{}, err
	}
	if st.RecoveryRequired {
		return domain.Project{}, domain.E(domain.Conflict, "interrupted operation requires state recover")
	}
	for _, p := range st.Registry.Projects {
		if p.Name != name {
			continue
		}
		if p.PHPCLI == (domain.PHPSelection{}) {
			return p, domain.E(domain.Unavailable, "project has no explicit PHP CLI selection")
		}
		now, err := domain.ResolveProject(p.Name, p.Root, p.DocumentRoot)
		if err != nil {
			return p, err
		}
		now.PHPCLI = p.PHPCLI
		if now != p {
			return p, domain.E(domain.Conflict, "project directory identity changed; relink before execution")
		}
		return p, nil
	}
	return domain.Project{}, domain.E(domain.Unavailable, "project is not linked")
}

func (s RuntimeService) Resolve(name string) (ExecutionSpec, error) {
	p, err := s.project(name)
	if err != nil {
		return ExecutionSpec{}, err
	}
	f, err := phpcli.Open(p.PHPCLI)
	if err != nil {
		return ExecutionSpec{}, err
	}
	f.Close()
	return ExecutionSpec{Executable: p.PHPCLI, Directory: p.Root, Environment: map[string]string{"VELNIX_PHP": p.PHPCLI.Path, "PWD": p.Root}}, nil
}

// ChildExit distinguishes a child status from Velnix's diagnostic exit codes.
type ChildExit struct{ Code int }

func (e *ChildExit) Error() string { return "PHP command exited unsuccessfully" }

// Exec invokes the selected PHP directly, with no shell or project hooks. User
// PHP ini and application environment are intentionally available for this
// explicit user-code invocation, unlike the isolated version probe.
func (s RuntimeService) Exec(ctx context.Context, name string, args []string, in io.Reader, out, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := s.project(name)
	if err != nil {
		return err
	}
	f, err := phpcli.Open(p.PHPCLI)
	if err != nil {
		return err
	}
	defer f.Close()
	c := phpcli.Command(f, p.PHPCLI, args...)
	c.Dir = p.Root
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "VELNIX_PHP=") && !strings.HasPrefix(variable, "PWD=") {
			c.Env = append(c.Env, variable)
		}
	}
	c.Env = append(c.Env, "VELNIX_PHP="+p.PHPCLI.Path, "PWD="+p.Root)
	c.Stdin, c.Stdout, c.Stderr = in, out, stderr
	if err = c.Start(); err != nil {
		return domain.E(domain.Unavailable, "cannot start selected PHP: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		sig := os.Signal(syscall.SIGTERM)
		var cause *process.SignalCause
		if errors.As(context.Cause(ctx), &cause) {
			sig = cause.Signal
		}
		_ = c.Process.Signal(sig)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			_ = c.Process.Kill()
			<-done
		}
		return ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			code = 128 + int(status.Signal())
		}
		return &ChildExit{Code: code}
	}
	return err
}
