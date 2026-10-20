//go:build linux

package app

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"velnix/internal/domain"
	"velnix/internal/process"
	"velnix/internal/projectmeta"
	"velnix/internal/providers/managedstore"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/runtimeinventory"
	"velnix/internal/state"
)

type ProviderCapability struct {
	Provider   string   `json:"provider"`
	Kinds      []string `json:"kinds"`
	Resolution bool     `json:"resolution"`
	Reason     string   `json:"reason"`
}

func RuntimeProviders() []ProviderCapability {
	return []ProviderCapability{
		{"native", []string{"php", "node", "bun", "python"}, true, "Existing absolute native executables only; explicit approved exact-version probe, passive resolution; no ownership or installation grant."},
		{"mise", []string{"node", "bun"}, true, "Read-only existing-store-v1 adapter: explicit mise data root, exact installs/KIND/VERSION/bin/KIND link; no manager commands, downloads or hooks."},
		{"uv", []string{"python"}, true, "Read-only existing-store-v1 adapter: explicit uv Python root, exact CPython Linux amd64/arm64 GNU layout; no manager commands, downloads or hooks."},
		{"nvm/pyenv", []string{"node", "python"}, false, "Shell profiles and manager shims are not executed. Supply the absolute existing native installation."},
	}
}
func (s RuntimeService) linked(name string) (domain.Project, error) {
	if !domain.ValidProjectName(name) {
		return domain.Project{}, domain.E(domain.Invalid, "invalid project name")
	}
	st, e := s.Store.Status()
	if e != nil {
		return domain.Project{}, e
	}
	if st.RecoveryRequired {
		return domain.Project{}, domain.E(domain.Conflict, "recover registry first")
	}
	for _, p := range st.Registry.Projects {
		if p.Name == name {
			now, e := domain.ResolveProject(name, p.Root, p.DocumentRoot)
			if e != nil {
				return p, e
			}
			if now.WithSettingsFrom(p) != p {
				return p, domain.E(domain.Conflict, "project identity changed; relink first")
			}
			return p, nil
		}
	}
	return domain.Project{}, domain.E(domain.Unavailable, "project is not linked")
}
func pythonVenv(p domain.Project, selection domain.ExecutableSelection, path string) (domain.RuntimeSelection, error) {
	result := domain.RuntimeSelection{Executable: selection}
	if path == "" {
		candidate := filepath.Join(p.Root, ".venv")
		if _, e := os.Lstat(candidate); os.IsNotExist(e) {
			return result, nil
		} else if e != nil {
			return result, e
		}
		path = candidate
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return result, domain.E(domain.Invalid, "virtualenv must be an absolute clean directory")
	}
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil {
		return result, e
	}
	if canonical != path {
		return result, domain.E(domain.Denied, "virtualenv directory aliases are unsupported")
	}
	rel, e := filepath.Rel(p.Root, path)
	if e != nil || !domain.ValidRelativeMetadata(rel) {
		return result, domain.E(domain.Invalid, "virtualenv must be inside the linked project")
	}
	info, e := os.Stat(path)
	if e != nil {
		return result, e
	}
	if !info.IsDir() {
		return result, domain.E(domain.Invalid, "virtualenv must be a directory")
	}
	identity := info.Sys().(*syscall.Stat_t)
	b, e := projectmeta.Read(p.Root, filepath.Join(rel, "pyvenv.cfg"))
	if e != nil {
		return result, domain.E(domain.Unavailable, "virtualenv metadata unavailable: %v", e)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return result, domain.E(domain.Invalid, "invalid pyvenv.cfg")
		}
		key = strings.TrimSpace(key)
		if _, ok := values[key]; ok {
			return result, domain.E(domain.Invalid, "duplicate pyvenv.cfg setting")
		}
		values[key] = strings.TrimSpace(value)
	}
	home, e := filepath.EvalSymlinks(values["home"])
	if e != nil || home != filepath.Dir(selection.Path) || values["version"] != selection.Version || values["include-system-site-packages"] != "false" {
		return result, domain.E(domain.Conflict, "virtualenv home/version must match selected interpreter and system-site-packages must be false")
	}
	launcher := filepath.Join(path, "bin", "python")
	actual, e := filepath.EvalSymlinks(launcher)
	if e != nil || actual != selection.Path {
		return result, domain.E(domain.Conflict, "virtualenv interpreter differs from selected exact executable")
	}
	// Reject a symlinked bin directory; only the interpreter leaf may be an alias.
	bin, e := os.Lstat(filepath.Dir(launcher))
	if e != nil || !bin.IsDir() {
		return result, domain.E(domain.Denied, "virtualenv bin must be a real directory")
	}
	result.Venv = path
	result.VenvConfig = hash(b)
	result.VenvDevice = uint64(identity.Dev)
	result.VenvInode = identity.Ino
	result.Launcher = launcher
	return result, result.Validate("python")
}
func (s RuntimeService) multi(ctx context.Context, req RuntimeRequest) (any, error) {
	if !domain.MultiRuntime(req.Kind) {
		return nil, domain.E(domain.Invalid, "kind must be node, bun or python")
	}
	p, e := s.linked(req.Name)
	if e != nil {
		return nil, e
	}
	switch req.Action {
	case "resolve":
		return s.ResolveKind(req.Name, req.Kind)
	case "select", "clear":
		change := state.ProjectChange{Action: "clear-runtime", Name: req.Name, Kind: req.Kind}
		if req.Action == "select" {
			if req.Provider != "" || req.ProviderRoot != "" {
				source, err := managedstore.Resolve(managedstore.Request{Provider: req.Provider, Root: req.ProviderRoot, Kind: req.Kind, Version: req.Version})
				if err != nil {
					return nil, err
				}
				change.Provider = req.Provider
				change.ProviderRoot = source.Root
				req.Path = source.Executable.Path
			}
			executable, e := nativeexec.Snapshot(req.Path, req.Version)
			if e != nil {
				return nil, e
			}
			report, e := projectmeta.Inspect(p.Root)
			if e != nil {
				return nil, e
			}
			if e = report.Check(req.Kind, req.Version); e != nil {
				return nil, e
			}
			selected := domain.RuntimeSelection{Executable: executable}
			if req.Kind == "python" {
				selected, e = pythonVenv(p, executable, req.Venv)
				if e != nil {
					return nil, e
				}
			} else if req.Venv != "" {
				return nil, domain.E(domain.Invalid, "--venv requires Python")
			}
			if _, e = s.checkMulti(p, req.Kind, selected); e != nil {
				return nil, e
			}
			change.MetadataDigest = report.Digest
			change.Action = "select-runtime"
			change.Runtime = selected
		}
		plan, e := s.Store.PlanProject(change)
		if e != nil {
			return nil, e
		}
		if req.Approval == "" {
			return plan, nil
		}
		if req.Approval != plan.ID {
			return nil, domain.E(domain.Conflict, "runtime plan changed; preview again")
		}
		if req.Action == "select" && len(plan.Effects) > 0 {
			if e = runtimeinventory.Probe(ctx, req.Kind, change.Runtime.Executable); e != nil {
				return nil, e
			}
		}
		// Recheck metadata after the approved executable ran, without executing hooks.
		if req.Action == "select" {
			if _, e = s.checkMulti(p, req.Kind, change.Runtime); e != nil {
				return nil, e
			}
		}
		return s.Store.ApplyProject(ctx, change, req.Approval)
	default:
		return nil, domain.E(domain.Invalid, "unknown runtime action")
	}
}
func (s RuntimeService) checkMulti(p domain.Project, kind string, selected domain.RuntimeSelection) (map[string]string, error) {
	if selected == (domain.RuntimeSelection{}) {
		return nil, domain.E(domain.Unavailable, "project has no explicit %s selection", kind)
	}
	if e := selected.Validate(kind); e != nil {
		return nil, e
	}
	f, e := nativeexec.Open(selected.Executable)
	if e != nil {
		return nil, e
	}
	f.Close()
	report, e := projectmeta.Inspect(p.Root)
	if e != nil {
		return nil, e
	}
	if e = report.Check(kind, selected.Executable.Version); e != nil {
		return nil, e
	}
	bin := filepath.Dir(selected.Executable.Path)
	if kind == "node" || kind == "bun" {
		alias, e := nativeexec.Snapshot(filepath.Join(bin, kind), selected.Executable.Version)
		if e != nil || alias != selected.Executable {
			return nil, domain.E(domain.Unavailable, "selected %s directory must expose the matching %s command for child PATH consistency", kind, kind)
		}
	}
	env := map[string]string{"PWD": p.Root, "VELNIX_" + strings.ToUpper(kind): selected.Executable.Path}
	if kind == "python" {
		now, e := pythonVenv(p, selected.Executable, selected.Venv)
		if e != nil {
			return nil, e
		}
		if now != selected {
			return nil, domain.E(domain.Conflict, "virtualenv appeared or changed; review a fresh Python selection")
		}
		if selected.Venv != "" {
			bin = filepath.Dir(selected.Launcher)
			env["VIRTUAL_ENV"] = selected.Venv
			env["VELNIX_PYTHON"] = selected.Launcher
		}
	}
	if strings.Contains(bin, ":") {
		return nil, domain.E(domain.Invalid, "runtime directory cannot contain a PATH separator")
	}
	env["PATH"] = bin + ":/usr/bin:/bin"
	return env, nil
}
func (s RuntimeService) ResolveKind(name, kind string) (ExecutionSpec, error) {
	if kind == "" || kind == "php" {
		return s.Resolve(name)
	}
	p, e := s.linked(name)
	if e != nil {
		return ExecutionSpec{}, e
	}
	selected := p.Runtime(kind)
	env, e := s.checkMulti(p, kind, selected)
	if e != nil {
		return ExecutionSpec{}, e
	}
	return ExecutionSpec{Executable: selected.Executable, Directory: p.Root, Environment: env, UnsetPrefixes: []string{"NODE_", "BUN_", "PYTHON", "LD_", "DYLD_", "VELNIX_"}, UnsetVariables: []string{"VIRTUAL_ENV", "_OLD_VIRTUAL_PATH"}}, nil
}
func (s RuntimeService) ExecKind(ctx context.Context, name, kind string, args []string, in io.Reader, out, stderr io.Writer) error {
	if kind == "" || kind == "php" {
		return s.Exec(ctx, name, args, in, out, stderr)
	}
	if !domain.MultiRuntime(kind) {
		return domain.E(domain.Invalid, "kind must be php, node, bun or python")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	p, e := s.linked(name)
	if e != nil {
		return e
	}
	selected := p.Runtime(kind)
	env, e := s.checkMulti(p, kind, selected)
	if e != nil {
		return e
	}
	f, e := nativeexec.Open(selected.Executable)
	if e != nil {
		return e
	}
	defer f.Close()
	if kind == "bun" {
		args = append([]string{"--no-install"}, args...)
	}
	cmd := nativeexec.Command(f, selected.Executable, args...)
	if selected.Venv != "" {
		cmd.Args[0] = selected.Launcher
	}
	cmd.Dir = p.Root
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, replaced := env[key]; replaced {
			continue
		}
		if strings.HasPrefix(key, "NODE_") || strings.HasPrefix(key, "BUN_") || strings.HasPrefix(key, "PYTHON") || strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_") || key == "VIRTUAL_ENV" || key == "_OLD_VIRTUAL_PATH" || strings.HasPrefix(key, "VELNIX_") {
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, stderr
	return runSelected(ctx, cmd)
}
func runSelected(ctx context.Context, c *exec.Cmd) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := c.Start(); e != nil {
		return domain.E(domain.Unavailable, "cannot start selected runtime: %v", e)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		sig := os.Signal(syscall.SIGTERM)
		var cause *process.SignalCause
		if errors.As(context.Cause(ctx), &cause) {
			sig = cause.Signal
		}
		c.Process.Signal(sig)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			c.Process.Kill()
			<-done
		}
		return ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if st, ok := exit.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			code = 128 + int(st.Signal())
		}
		return &ChildExit{Code: code}
	}
	return err
}
