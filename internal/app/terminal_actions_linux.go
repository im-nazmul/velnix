package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"velnix/internal/domain"
	"velnix/internal/state"
)

// TerminalChange is a bounded operation, never a shell command.
type TerminalChange struct {
	Operation    string `json:"operation"`
	Name         string `json:"project,omitempty"`
	Root         string `json:"root,omitempty"`
	DocumentRoot string `json:"document_root,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Executable   string `json:"executable,omitempty"`
	Version      string `json:"version,omitempty"`
	Profile      string `json:"profile,omitempty"`
	Uploads      string `json:"upload_directories,omitempty"`
	Port         int    `json:"http_port,omitempty"`
	HTTPS        bool   `json:"https,omitempty"`
	HTTPSPort    int    `json:"https_port,omitempty"`
}
type TerminalPlan struct {
	ID, Detail string
	change     TerminalChange
}

// TerminalSession owns only processes explicitly launched from this TUI session.
// Closing it never signals previously running external owners.
type TerminalSession struct {
	Query  TerminalQuery
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	issues []error
	owners map[string]*terminalOwner
}

type terminalOwner struct{ finished chan struct{} }

func NewTerminalSession(ctx context.Context, q TerminalQuery) *TerminalSession {
	ctx, cancel := context.WithCancel(ctx)
	return &TerminalSession{Query: q, ctx: ctx, cancel: cancel, owners: make(map[string]*terminalOwner)}
}
func (s *TerminalSession) Read(ctx context.Context, screen int) []TerminalEntry {
	entries := s.Query.Read(ctx, screen)
	if screen == 0 || screen == 3 || screen == 4 {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, e := range s.issues {
			entries = append(entries, TerminalEntry{"Session workload needs attention", e.Error()})
		}
	}
	return entries
}
func (s *TerminalSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(s.issues...)
}
func (s *TerminalSession) run(ctx context.Context, c TerminalChange, id string) (any, error) {
	store := state.Store{Paths: s.Query.Paths}
	switch c.Operation {
	case "initialize":
		return (StateService{Store: store}).Run(ctx, StateRequest{Action: "init", ApprovedPlanID: id})
	case "link", "unlink":
		return (ProjectService{Store: store}).Run(ctx, ProjectRequest{ProjectChange: state.ProjectChange{Action: c.Operation, Name: c.Name, Root: c.Root, DocumentRoot: c.DocumentRoot}, ApprovedPlanID: id})
	case "select-runtime", "clear-runtime":
		action := "select"
		if c.Operation == "clear-runtime" {
			action = "clear"
		}
		return (RuntimeService{Store: store}).Run(ctx, RuntimeRequest{Action: action, Name: c.Name, Kind: c.Kind, Path: c.Executable, Version: c.Version, Approval: id})
	case "start-http", "stop-http", "reload-http", "start-fpm", "stop-fpm":
		parts := strings.Split(c.Operation, "-")
		service := HTTPService{Store: store}
		if parts[1] == "fpm" {
			if !domain.ValidProjectName(c.Name) {
				return nil, domain.E(domain.Invalid, "FPM requires a linked project name")
			}
			service.FPMName = c.Name
		}
		req := HTTPRequest{Action: parts[0], Executable: c.Executable, Port: c.Port, Version: c.Version, Profile: c.Profile, Approval: id, HTTPS: c.HTTPS, HTTPSPort: c.HTTPSPort}
		if c.Uploads != "" {
			req.Uploads = strings.Split(c.Uploads, ",")
		}
		if id == "" {
			return service.Plan(ctx, req)
		}
		return service.Apply(ctx, req)
	default:
		return nil, domain.E(domain.Invalid, "unsupported terminal operation")
	}
}
func (s *TerminalSession) Plan(ctx context.Context, c TerminalChange) (TerminalPlan, error) {
	v, e := s.run(ctx, c, "")
	if e != nil {
		return TerminalPlan{}, e
	}
	var id string
	var review any = v
	switch p := v.(type) {
	case StateResult:
		if p.Plan != nil {
			id = p.Plan.ID
			review = p.Plan
		}
	case state.ProjectPlan:
		id = p.ID
		// The request and affected target are sufficient for this single-project
		// mutation. Avoid flooding review with unrelated registry records.
		var target any = "Entry removed"
		for _, project := range p.Registry.Projects {
			if project.Name == c.Name {
				target = project
			}
		}
		review = struct {
			Plan   state.Plan
			Target any
		}{p.Plan, target}
	case HTTPPlan:
		id = p.ID
	default:
		return TerminalPlan{}, domain.E(domain.Invalid, "operation did not produce a reviewable plan")
	}
	if id == "" {
		return TerminalPlan{}, domain.E(domain.Invalid, "empty approval ID")
	}
	b, e := json.MarshalIndent(review, "", "  ")
	if e != nil {
		return TerminalPlan{}, e
	}
	request, _ := json.MarshalIndent(c, "", "  ")
	note := "Only this exact reviewed plan can be applied. Changed state requires a fresh review."
	if c.Operation == "unlink" {
		note += " Source files are preserved. Existing web routes require a separate reviewed reload or stop to revoke access."
	}
	if strings.HasPrefix(c.Operation, "stop-") {
		note += " Stopping may interrupt active requests. Inspect web routing afterward."
	}
	if strings.HasPrefix(c.Operation, "select-") {
		note += " Applying runs the selected external executable for an approved version probe; libraries/configuration are not fully pinned."
	}
	if strings.HasPrefix(c.Operation, "start-") {
		note += " This foreground workload belongs to this TUI session and stops when you exit. No daemon or automatic restart."
	}
	if len(request)+len(b)+len(note) > 30000 {
		return TerminalPlan{}, domain.E(domain.Unavailable, "review exceeds terminal display limit; use the CLI to inspect this operation")
	}
	return TerminalPlan{ID: id, Detail: string(request) + "\n\n" + string(b) + "\n\n" + note, change: c}, nil
}
func (s *TerminalSession) Apply(ctx context.Context, p TerminalPlan) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if p.ID == "" {
		return domain.E(domain.Invalid, "review a plan before approval")
	}
	s.mu.Lock()
	if s.closed || s.ctx.Err() != nil {
		s.mu.Unlock()
		return context.Canceled
	}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	c := p.change
	key := "http"
	if strings.HasSuffix(c.Operation, "-fpm") {
		key = "fpm:" + c.Name
	}
	if !strings.HasPrefix(c.Operation, "start-") {
		_, e := s.run(ctx, c, p.ID)
		if e != nil {
			return e
		}
		if strings.HasPrefix(c.Operation, "stop-") {
			s.mu.Lock()
			owner := s.owners[key]
			s.mu.Unlock()
			if owner != nil {
				select {
				case <-owner.finished:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	}
	// Keep the existing foreground owner alive under the session context, while
	// returning readiness to the interface. Track it through cleanup on exit.
	ownerCtx, stop := context.WithCancel(s.ctx)
	owner := &terminalOwner{finished: make(chan struct{})}
	s.mu.Lock()
	if s.owners[key] != nil {
		s.mu.Unlock()
		stop()
		return domain.E(domain.Conflict, "session workload is still active or cleaning up; inspect status before starting again")
	}
	s.owners[key] = owner
	s.wg.Add(1)
	s.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		defer s.wg.Done()
		defer func() { s.mu.Lock(); delete(s.owners, key); close(owner.finished); s.mu.Unlock() }()
		_, e := s.run(ownerCtx, c, p.ID)
		if e != nil && !errors.Is(e, context.Canceled) {
			s.mu.Lock()
			s.issues = append(s.issues, e)
			s.mu.Unlock()
		}
		done <- e
		stop()
	}()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	service := HTTPService{Store: state.Store{Paths: s.Query.Paths}}
	if c.Operation == "start-fpm" {
		service.FPMName = c.Name
	}
	for {
		select {
		case e := <-done:
			return e
		case <-ctx.Done():
			stop()
			return ctx.Err()
		case <-deadline.C:
			stop()
			return fmt.Errorf("workload startup timed out; inspect service status before retrying")
		case <-ticker.C:
			st, e := service.Status()
			if e == nil && st.Record != nil && st.Record.PlanID == p.ID && st.Record.Phase == "running" && st.Alive {
				return nil
			}
		}
	}
}
