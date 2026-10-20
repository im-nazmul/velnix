//go:build linux

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"velnix/internal/domain"
	"velnix/internal/process"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/nginx"
	"velnix/internal/providers/phpfpm"
	"velnix/internal/state"
)

type HTTPRequest struct {
	HTTPS                        bool
	HTTPSPort                    int
	Action, Executable, Approval string
	Port                         int
	DryRun                       bool
	Version, Profile             string
	Uploads                      []string
}
type HTTPRecord struct {
	Upstreams      []int              `json:"upstreams,omitempty"`
	SocketDevice   uint64             `json:"socket_device,omitempty"`
	SocketInode    uint64             `json:"socket_inode,omitempty"`
	TLS            *TLSRecord         `json:"tls,omitempty"`
	HTTPSPort      int                `json:"https_port,omitempty"`
	Schema         int                `json:"schema"`
	FPM            *phpfpm.Config     `json:"fpm,omitempty"`
	Pools          []PoolEvidence     `json:"pools,omitempty"`
	Phase          string             `json:"phase"`
	Process        process.Identity   `json:"process"`
	Workers        []process.Identity `json:"workers,omitempty"`
	Executable     string             `json:"executable"`
	Port           int                `json:"port"`
	RegistryDigest string             `json:"registry_digest"`
	ConfigDigest   string             `json:"config_digest"`
	Token          string             `json:"health_token"`
	PlanID         string             `json:"plan_id"`
}
type HTTPStatus struct {
	RoutingError     string      `json:"routing_error,omitempty"`
	Record           *HTTPRecord `json:"record"`
	Alive            bool        `json:"alive"`
	PendingRouting   bool        `json:"pending_routing"`
	RecoveryRequired bool        `json:"recovery_required"`
}
type HTTPPlan struct {
	ID               string     `json:"id"`
	Action           string     `json:"action"`
	Prefix           string     `json:"prefix"`
	Registry         string     `json:"registry_digest"`
	Before           string     `json:"record_before"`
	ConfigBefore     string     `json:"config_before"`
	ExecutableDigest string     `json:"executable_digest"`
	Target           HTTPRecord `json:"target"`
	Configuration    string     `json:"configuration"`
	Effects          []string   `json:"effects"`
	Limitations      string     `json:"limitations"`
}

// FPMName selects a per-project foreground pool using the same transition engine.
type HTTPService struct {
	Store   state.Store
	FPMName string
}
type PoolEvidence struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

func (s HTTPService) filename(name string) string {
	if s.FPMName != "" {
		return "fpm-" + s.FPMName + "-" + name
	}
	return name
}
func (s HTTPService) read() ([]byte, error) {
	if s.FPMName != "" && !domain.ValidProjectName(s.FPMName) {
		return nil, domain.E(domain.Invalid, "invalid FPM project name")
	}
	return s.Store.ReadWorkload(s.filename("http.json"))
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func data(v any) []byte    { b, _ := json.Marshal(v); return b }
func readRecord(b []byte) (*HTTPRecord, error) {
	if b == nil {
		return nil, nil
	}
	var r HTTPRecord
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&r); e != nil {
		return nil, e
	}
	if e := decoder.Decode(new(any)); e != io.EOF {
		return nil, domain.E(domain.Invalid, "HTTP record contains trailing JSON")
	}
	if r.Schema != 1 {
		return nil, domain.E(domain.Invalid, "unsupported HTTP record schema")
	}
	switch r.Phase {
	case "applying", "running", "stopped", "recovery-required":
	default:
		return nil, domain.E(domain.Invalid, "invalid HTTP phase")
	}
	return &r, nil
}
func (s HTTPService) Status() (HTTPStatus, error) {
	b, e := s.read()
	if e != nil {
		return HTTPStatus{}, e
	}
	r, e := readRecord(b)
	if e != nil {
		return HTTPStatus{}, e
	}
	st := HTTPStatus{Record: r}
	if r == nil {
		return st, nil
	}
	st.Alive, e = process.CheckedAlive(r.Process)
	if e != nil {
		return st, e
	}
	registry, e := s.Store.Status()
	if e != nil {
		return st, e
	}
	st.PendingRouting = r.RegistryDigest != hash(data(registry.Registry))
	if s.FPMName == "" {
		_, pools, err := s.routes(registry.Registry.Projects)
		if err != nil {
			st.RoutingError = err.Error()
			st.PendingRouting = true
		} else if !reflect.DeepEqual(r.Pools, pools) {
			st.PendingRouting = true
		}
		if r.TLS != nil {
			current, err := (TLSService{Store: s.Store}).Status()
			if err != nil {
				st.RoutingError = err.Error()
				st.PendingRouting = true
			} else if !reflect.DeepEqual(r.TLS, current) {
				st.PendingRouting = true
			}
		}
	}
	st.RecoveryRequired = r.Phase == "applying" || r.Phase == "recovery-required" || (r.Phase == "running" && !st.Alive)
	return st, nil
}
func executableDigest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	if info.Size() > 128<<20 {
		return "", domain.E(domain.Invalid, "Nginx executable exceeds 128 MiB limit")
	}
	if _, e = io.Copy(h, io.LimitReader(f, (128<<20)+1)); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (s HTTPService) plan(req HTTPRequest, f *state.HTTPFiles) (HTTPPlan, error) {
	var p HTTPPlan
	if req.Action != "start" && req.Action != "reload" && req.Action != "stop" && req.Action != "recover" && !(s.FPMName != "" && req.Action == "disable") {
		return p, domain.E(domain.Invalid, "unknown HTTP action")
	}
	st, e := s.Store.Status()
	if e != nil {
		return p, e
	}
	if !st.Initialized || (st.RecoveryRequired && (req.Action == "start" || req.Action == "reload")) {
		return p, domain.E(domain.Conflict, "initialize or recover registry state first")
	}
	b, e := s.read()
	if e != nil {
		return p, e
	}
	r, e := readRecord(b)
	if e != nil {
		return p, e
	}
	conf, e := f.Read(s.filename("http.conf"))
	if e != nil {
		return p, e
	}
	if r == nil && conf != nil {
		return p, domain.E(domain.Conflict, "HTTP configuration exists without ownership evidence; preserve it for manual review")
	}
	p = HTTPPlan{Action: req.Action, Prefix: s.Store.Paths.State, Registry: hash(data(st.Registry)), Before: hash(b), ConfigBefore: hash(conf), Effects: []string{}, Limitations: "Current-user loopback workloads only. FPM uses installed default PHP ini/extensions with a minimal environment; their dependencies are not pinned. Source remains external. Unlink requires reload or stop to revoke active routing. Recovery never replays a launch; unknown edits require manual review."}
	live := false
	if r != nil {
		live, e = process.CheckedAlive(r.Process)
		if e != nil {
			return p, e
		}
	}
	if r != nil && (r.Phase == "applying" || r.Phase == "recovery-required" || (r.Phase == "running" && !live)) && req.Action != "recover" && req.Action != "stop" {
		return p, domain.E(domain.Conflict, "HTTP recovery required; inspect http status and preview http recover")
	}
	if req.Action == "start" && live {
		return p, domain.E(domain.Conflict, "owned HTTP instance is already active")
	}
	if req.Action == "reload" && (!live || r == nil || r.Phase != "running") {
		return p, domain.E(domain.Unavailable, "no healthy owned HTTP process to reload")
	}
	if r != nil && r.ConfigDigest != "" && hash(conf) != r.ConfigDigest {
		return p, domain.E(domain.Conflict, "HTTP configuration changed externally; preserve evidence for manual review")
	}
	if req.Action == "stop" || req.Action == "recover" || req.Action == "disable" {
		if r == nil {
			p.ID = hash(data(p))
			return p, nil
		}
		p.Target = *r
		p.Target.Phase = "stopped"
		if req.Action == "disable" {
			if live || r.Phase != "stopped" {
				return p, domain.E(domain.Conflict, "stop/recover FPM before disabling PHP routing")
			}
			p.Target.FPM = nil
			p.Effects = []string{"disable PHP routing for future HTTP start/reload; active routes require reload"}
		}
		if r.Phase == "applying" && r.Process.PID == 0 {
			return p, domain.E(domain.Conflict, "interrupted launch has no process identity; inspect owned prefix/processes manually before reconciling http.json")
		}
		hasWorkers := false
		for _, worker := range r.Workers {
			hasWorkers = hasWorkers || process.Alive(worker)
		}
		if req.Action != "disable" {
			if live || hasWorkers {
				p.Effects = []string{"journal stop intent", "signal verified owned master and wait for shutdown", "mark workload stopped"}
			} else if r.Phase != "stopped" {
				p.Effects = []string{"mark exited workload stopped; never relaunch"}
			}
		}
	} else {
		if os.Geteuid() == 0 {
			return p, domain.E(domain.Denied, "HTTP workloads must run as an unprivileged user")
		}
		probe, e := process.Probe()
		if e != nil {
			return p, domain.E(domain.Unavailable, "safe HTTP lifecycle requires Linux pidfds: %v", e)
		}
		unix.Close(probe)
		exe := req.Executable
		port := req.Port
		if req.Action == "reload" {
			exe = r.Executable
			port = r.Port
		}
		var pool *phpfpm.Config
		if s.FPMName != "" {
			if !domain.ValidProjectName(s.FPMName) {
				return p, domain.E(domain.Invalid, "invalid FPM project name")
			}
			var project *domain.Project
			for _, item := range st.Registry.Projects {
				if item.Name == s.FPMName {
					v := item
					project = &v
				}
			}
			if project != nil && project.ProxyPort != 0 {
				return p, domain.E(domain.Conflict, "disable proxy before starting FPM")
			}
			if project == nil {
				return p, domain.E(domain.Unavailable, "linked project not found")
			}
			selection, err := nativeexec.Snapshot(exe, req.Version)
			if err != nil {
				return p, err
			}
			profile := req.Profile
			if profile == "" {
				profile = "php"
			}
			pool = &phpfpm.Config{Project: *project, Selection: selection, Profile: profile, Uploads: req.Uploads, Socket: filepath.Join(p.Prefix, "fpm-"+s.FPMName+".sock")}
			if err = pool.Validate(); err != nil {
				return p, err
			}
			exe = selection.Path
			p.ExecutableDigest = selection.SHA256
			if req.Action == "reload" {
				return p, domain.E(domain.Invalid, "FPM reload is unavailable; stop and review a fresh start")
			}
		} else {
			exe, e = nginx.Executable(exe)
			if e != nil {
				return p, e
			}
			p.ExecutableDigest, e = executableDigest(exe)
			if e != nil {
				return p, e
			}
		}

		for _, project := range st.Registry.Projects {
			now, e := domain.ResolveProject(project.Name, project.Root, project.DocumentRoot)
			if e != nil {
				return p, e
			}
			now = now.WithSettingsFrom(project)
			if now != project {
				return p, domain.E(domain.Conflict, "project directory identity changed; relink %s", project.Name)
			}
		}
		p.Target = HTTPRecord{FPM: pool, Schema: 1, Phase: "running", Executable: exe, Port: port, RegistryDigest: p.Registry, Token: hash(data([]string{p.Registry, p.Before, p.ConfigBefore, req.Action}))}
		if r != nil && req.Action == "reload" {
			p.Target.Process = r.Process
			p.Target.Workers = r.Workers
		}
		if pool != nil {
			if _, err := os.Lstat(pool.Socket); err == nil {
				return p, domain.E(domain.Conflict, "FPM socket already exists; preserve it and inspect recovery")
			} else if !os.IsNotExist(err) {
				return p, err
			}
			p.Configuration, e = phpfpm.Render(*pool, p.Prefix, "fpm-"+s.FPMName)
		} else {
			routes, pools, err := s.routes(st.Registry.Projects)
			if err != nil {
				return p, err
			}
			p.Target.Pools = pools
			for _, project := range st.Registry.Projects {
				if project.ProxyPort != 0 {
					p.Target.Upstreams = append(p.Target.Upstreams, project.ProxyPort)
				}
			}
			p.Configuration, e = nginx.RenderWeb(st.Registry.Projects, port, p.Target.Token, p.Prefix, routes)
			tlsEnabled, tlsPort := req.HTTPS, req.HTTPSPort
			if req.Action == "reload" && r.TLS != nil {
				tlsEnabled = true
				tlsPort = r.HTTPSPort
			}
			if tlsEnabled {
				if tlsPort < 1024 || tlsPort > 65535 || tlsPort == port {
					return p, domain.E(domain.Invalid, "HTTPS port must be 1024–65535 and differ from HTTP")
				}
				cert, err := (TLSService{Store: s.Store}).materialLocked(*f)
				if err != nil {
					return p, err
				}
				names := []string{}
				for _, project := range st.Registry.Projects {
					names = append(names, project.Domain)
				}
				sort.Strings(names)
				if !reflect.DeepEqual(names, cert.Names) {
					return p, domain.E(domain.Conflict, "registered TLS names changed; issue a fresh certificate")
				}
				p.Target.TLS = cert
				p.Target.HTTPSPort = tlsPort
				for _, project := range st.Registry.Projects {
					if project.ProxyPort == tlsPort {
						return p, domain.E(domain.Invalid, "proxy target cannot be the HTTPS listener")
					}
				}
				p.Configuration, e = nginx.WithTLS(p.Configuration, port, tlsPort, filepath.Join(p.Prefix, cert.Generation, "cert.pem"), filepath.Join(p.Prefix, cert.Generation, "key.pem"))
			}
		}
		if e != nil {
			return p, e
		}
		if len(p.Configuration) > 1024*1024 {
			return p, domain.E(domain.Invalid, "generated HTTP configuration exceeds 1 MiB limit")
		}
		if pool != nil {
			p.Limitations += " Foreground FPM validates exact version/SAPI using -n -v, then validates/starts with -y CONFIG -F using installed PHP ini defaults. No CLI selection or global services change. Only supplied upload directories plus built-in sensitive paths are denied; use a dedicated public root."
		}
		p.Target.ConfigDigest = hash([]byte(p.Configuration))
		p.Effects = []string{"write private candidate and validate with selected executable", "journal intent and atomically publish generated configuration", "start foreground owned process or signal verified master", "verify process and HTTP generation; record running state"}
	}
	p.ID = hash(data(p))
	return p, nil
}
func (s HTTPService) Plan(ctx context.Context, req HTTPRequest) (HTTPPlan, error) {
	if req.Action != "start" && req.Action != "stop" && req.Action != "reload" && req.Action != "recover" && !(s.FPMName != "" && req.Action == "disable") {
		return HTTPPlan{}, domain.E(domain.Invalid, "unknown HTTP action")
	}
	registry, e := s.Store.Status()
	if e != nil {
		return HTTPPlan{}, e
	}
	if !registry.Initialized {
		return HTTPPlan{}, domain.E(domain.Unavailable, "initialize state before managing HTTP")
	}
	var p HTTPPlan
	e = s.Store.WithHTTP(ctx, func(f state.HTTPFiles) error { var e error; p, e = s.plan(req, &f); return e })
	return p, e
}
func health(ctx context.Context, r HTTPRecord) error {
	if r.FPM != nil {
		return poolHealth(ctx, r)
	}

	client := http.Client{Timeout: 200 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	for {
		if !process.Alive(r.Process) {
			return domain.E(domain.Unavailable, "owned Nginx process exited")
		}
		request, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/", r.Port), nil)
		request.Host = "velnix-health.invalid"
		resp, e := client.Do(request)
		if e == nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
			resp.Body.Close()
			if resp.StatusCode == 200 && string(b) == r.Token {
				if r.TLS != nil {
					return tlsHealth(ctx, r)
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
func shutdown(r HTTPRecord) error {
	workers := append([]process.Identity(nil), r.Workers...)
	alive, e := process.CheckedAlive(r.Process)
	if e != nil {
		return e
	}
	if alive {
		current, e := process.Children(r.Process)
		if e != nil {
			return e
		}
		workers = append(workers, current...)
		if e = process.Signal(r.Process, unix.SIGTERM); e != nil {
			return e
		}
	} else {
		// A killed master can leave orphaned workers. Only signal previously
		// recorded identities, never a guessed PID or a process-name match.
		for _, worker := range workers {
			if process.Alive(worker) {
				if e := process.Signal(worker, unix.SIGTERM); e != nil {
					return e
				}
			}
		}
	}
	deadline := time.Now().Add(4 * time.Second)
	all := append(workers, r.Process)
	for _, identity := range all {
		for process.Alive(identity) {
			if time.Now().After(deadline) {
				return domain.E(domain.Conflict, "owned HTTP processes did not stop; recovery required")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return cleanupPoolSocket(r)
}

// Apply holds the mutation lock only during transitions, never over the
// foreground workload lifetime, so another CLI may explicitly reload or stop.
func (s HTTPService) Apply(ctx context.Context, req HTTPRequest) (HTTPStatus, error) {
	if req.DryRun {
		return HTTPStatus{}, domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
	}
	preview, err := s.Plan(ctx, req)
	if err != nil {
		return HTTPStatus{}, err
	}
	if preview.ID != req.Approval {
		return HTTPStatus{}, domain.E(domain.Conflict, "HTTP plan changed; preview again")
	}
	var owned *HTTPRecord
	var exited <-chan error
	e := s.Store.WithHTTP(ctx, func(f state.HTTPFiles) error {
		p, e := s.plan(req, &f)
		if e != nil {
			return e
		}
		if p.ID != req.Approval {
			return domain.E(domain.Conflict, "HTTP plan changed; preview again")
		}
		if len(p.Effects) == 0 {
			return nil
		}
		target := p.Target
		target.PlanID = p.ID
		write := func(phase string) error { target.Phase = phase; return f.Write(s.filename("http.json"), data(target)) }
		if req.Action == "stop" || req.Action == "recover" || req.Action == "disable" {
			if e = write("recovery-required"); e != nil {
				return e
			}
			if e = shutdown(target); e != nil {
				return e
			}
			return write("stopped")
		}
		step, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if req.Action == "start" && target.FPM == nil {
			listener, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", target.Port))
			if e != nil {
				return domain.E(domain.Conflict, "HTTP port is occupied or unavailable: %v", e)
			}
			listener.Close()
			if target.TLS != nil {
				l, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", target.HTTPSPort))
				if e != nil {
					return domain.E(domain.Conflict, "HTTPS port occupied or unavailable: %v", e)
				}
				l.Close()
			}
		}
		for _, port := range target.Upstreams {
			dialer := net.Dialer{Timeout: time.Second}
			conn, err := dialer.DialContext(step, "tcp4", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return domain.E(domain.Unavailable, "configured loopback app server on port %d is unavailable; start it explicitly first", port)
			}
			conn.Close()
		}
		if e = f.Write(s.filename("http-candidate.conf"), []byte(p.Configuration)); e != nil {
			return e
		}
		if target.FPM != nil {
			e = phpfpm.Version(step, *target.FPM)
			if e == nil {
				e = phpfpm.Validate(step, *target.FPM, filepath.Join(p.Prefix, s.filename("http-candidate.conf")))
			}
		} else {
			e = nginx.Validate(step, target.Executable, p.Prefix, filepath.Join(p.Prefix, s.filename("http-candidate.conf")))
		}
		if e != nil {
			return e
		}
		if e = step.Err(); e != nil {
			return e
		}
		// Keep exact prior evidence before changing a working generation.
		previous, e := f.Read(s.filename("http.conf"))
		if e != nil {
			return e
		}
		if e = f.Write(s.filename("http-before.conf"), previous); e != nil {
			return e
		}
		previousRecord, e := f.Read(s.filename("http.json"))
		if e != nil {
			return e
		}
		if e = f.Write(s.filename("http-before.json"), previousRecord); e != nil {
			return e
		}
		if e = write("applying"); e != nil {
			return e
		}
		if e = f.Write(s.filename("http.conf"), []byte(p.Configuration)); e != nil {
			return e
		}
		fail := func(cause error) error {
			_ = write("recovery-required")
			return fmt.Errorf("HTTP transition requires recovery: %w", cause)
		}
		var previousWorkers []process.Identity
		if req.Action == "reload" {
			previousWorkers, e = process.Children(target.Process)
			if e != nil {
				return fail(e)
			}
			if e = process.Signal(target.Process, unix.SIGHUP); e != nil {
				return fail(e)
			}
		} else {
			cmd := nginx.Command(target.Executable, p.Prefix, filepath.Join(p.Prefix, s.filename("http.conf")))
			var pinned *os.File
			if target.FPM != nil {
				pinned, e = nativeexec.Open(target.FPM.Selection)
				if e != nil {
					return fail(e)
				}
				defer pinned.Close()
				cmd = phpfpm.Command(pinned, *target.FPM, filepath.Join(p.Prefix, s.filename("http.conf")))
			}
			var tail nginx.Tail
			cmd.Stdout = &tail
			cmd.Stderr = &tail
			cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
			if e = cmd.Start(); e != nil {
				return fail(e)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			exited = done
			target.Process, e = process.Observe(cmd.Process.Pid)
			if e != nil {
				_ = cmd.Process.Kill()
				<-done
				return fail(e)
			}
			owned = &target
			if e = write("applying"); e != nil {
				_ = shutdown(target)
				return fail(e)
			}
		}
		if e = health(step, target); e != nil {
			if req.Action == "start" {
				_ = shutdown(target)
			}
			return fail(e)
		}
		// A new health response alone does not prove old workers have stopped
		// accepting requests for routes removed by this reload.
		for _, worker := range previousWorkers {
			for process.Alive(worker) {
				select {
				case <-step.Done():
					return fail(step.Err())
				case <-time.After(20 * time.Millisecond):
				}
			}
		}
		if target.FPM != nil {
			info, err := os.Lstat(target.FPM.Socket)
			if err != nil {
				return fail(err)
			}
			identity := info.Sys().(*syscall.Stat_t)
			target.SocketDevice = uint64(identity.Dev)
			target.SocketInode = identity.Ino
		}
		target.Workers, e = process.Children(target.Process)
		if e != nil {
			return fail(e)
		}
		return write("running")
	})
	if e != nil {
		if owned != nil {
			_ = shutdown(*owned)
		}
		return HTTPStatus{}, e
	}
	if owned != nil {
		var childErr error
		unexpected := false
		select {
		case <-ctx.Done():
		case childErr = <-exited:
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		e = s.Store.WithHTTP(cleanup, func(f state.HTTPFiles) error {
			b, e := f.Read(s.filename("http.json"))
			if e != nil {
				return e
			}
			r, e := readRecord(b)
			if e != nil {
				return e
			}
			if r == nil || r.Process != owned.Process {
				return domain.E(domain.Conflict, "foreground HTTP evidence changed; inspect recovery")
			}
			if e = shutdown(*r); e != nil {
				return e
			}
			unexpected = ctx.Err() == nil && r.Phase == "running"
			if unexpected {
				r.Phase = "recovery-required"
			} else if r.Phase == "running" || r.Phase == "stopped" {
				r.Phase = "stopped"
			} else {
				r.Phase = "recovery-required"
			}
			return f.Write(s.filename("http.json"), data(r))
		})
		if e != nil {
			return HTTPStatus{}, e
		}
		if ctx.Err() != nil {
			return HTTPStatus{}, ctx.Err()
		}
		if unexpected || childErr != nil {
			return HTTPStatus{}, domain.E(domain.Unavailable, "owned web workload exited unexpectedly: %v", childErr)
		}
	}
	return s.Status()
}

func tlsHealth(ctx context.Context, r HTTPRecord) error {
	if len(r.TLS.Names) == 0 {
		return domain.E(domain.Invalid, "TLS names missing")
	}
	roots := x509.NewCertPool()
	block, _ := pem.Decode([]byte(r.TLS.CAPEM))
	if block == nil {
		return domain.E(domain.Invalid, "TLS CA evidence missing")
	}
	ca, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return e
	}
	roots.AddCert(ca)
	dialer := tls.Dialer{Config: &tls.Config{RootCAs: roots, ServerName: r.TLS.Names[0], MinVersion: tls.VersionTLS12}}
	conn, e := dialer.DialContext(ctx, "tcp4", fmt.Sprintf("127.0.0.1:%d", r.HTTPSPort))
	if e != nil {
		return e
	}
	defer conn.Close()
	peer := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(peer) == 0 || hash(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peer[0].Raw})) != r.TLS.Digests["cert.pem"] {
		return domain.E(domain.Conflict, "HTTPS serves a different certificate")
	}
	return nil
}
