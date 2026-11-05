//go:build linux

package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	"golang.org/x/sys/unix"
	"velnix/internal/domain"
	"velnix/internal/process"
	"velnix/internal/providers/mailpit"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/state"
)

type MailRequest struct {
	Action, Executable, ID, Attachment, Output, Approval string
	Policy                                               mailpit.Policy
	Offset                                               int
	DryRun                                               bool
}
type MailRecord struct {
	Schema    int                        `json:"schema"`
	Phase     string                     `json:"phase"`
	PlanID    string                     `json:"plan_id"`
	Selection domain.ExecutableSelection `json:"selection"`
	Policy    mailpit.Policy             `json:"policy"`
	Parent    process.Identity           `json:"parent"`
	Process   process.Identity           `json:"process"`
	Session   string                     `json:"session"`
}
type MailStatus struct {
	LastOperation    *MailOperation `json:"last_operation,omitempty"`
	Record           *MailRecord    `json:"record"`
	Alive            bool           `json:"alive"`
	RecoveryRequired bool           `json:"recovery_required"`
	Disk             state.MailDisk `json:"disk"`
	Available        bool           `json:"available"`
	Reason           string         `json:"reason,omitempty"`
}
type MailPlan struct {
	ID              string                     `json:"id"`
	Action          string                     `json:"action"`
	OperationBefore string                     `json:"operation_before"`
	Before          string                     `json:"record_before"`
	Disk            state.MailDisk             `json:"disk"`
	Selection       domain.ExecutableSelection `json:"selection,omitempty"`
	Policy          mailpit.Policy             `json:"policy"`
	Runtime         string                     `json:"runtime_base,omitempty"`
	MessageID       string                     `json:"message_id,omitempty"`
	Attachment      string                     `json:"attachment_id,omitempty"`
	ContentDigest   string                     `json:"content_digest,omitempty"`
	Export          *state.ExportLocation      `json:"export,omitempty"`
	Candidates      []string                   `json:"candidates,omitempty"`
	Snapshot        string                     `json:"snapshot_digest,omitempty"`
	Effects         []string                   `json:"effects"`
	Limitations     string                     `json:"limitations"`
}
type MailOperation struct {
	Schema    int      `json:"schema"`
	Plan      MailPlan `json:"plan"`
	Phase     string   `json:"phase"`
	Completed int      `json:"completed"`
}
type MailService struct {
	Store state.Store
	Ready func(MailRecord) error
}

func readMailOperation(b []byte) (*MailOperation, error) {
	if b == nil {
		return nil, nil
	}
	var op MailOperation
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(&op); e != nil {
		return nil, domain.E(domain.Invalid, "invalid mail operation evidence")
	}
	if d.Decode(new(any)) != io.EOF || op.Schema != 1 || op.Plan.ID == "" {
		return nil, domain.E(domain.Invalid, "unsupported mail operation evidence")
	}
	unsigned := op.Plan
	unsigned.ID = ""
	if hash(data(unsigned)) != op.Plan.ID {
		return nil, domain.E(domain.Conflict, "mail operation plan digest changed; preserve evidence for manual review")
	}
	switch op.Phase {
	case "applying", "complete", "recovery-required":
	default:
		return nil, domain.E(domain.Invalid, "invalid mail operation phase")
	}
	return &op, nil
}

func mailRecord(b []byte) (*MailRecord, error) {
	if b == nil {
		return nil, nil
	}
	var r MailRecord
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(&r); e != nil {
		return nil, domain.E(domain.Invalid, "invalid mail lifecycle record")
	}
	if d.Decode(new(any)) != io.EOF || r.Schema != 1 {
		return nil, domain.E(domain.Invalid, "unsupported mail lifecycle record")
	}
	switch r.Phase {
	case "applying", "running", "stopped":
	default:
		return nil, domain.E(domain.Invalid, "invalid mail lifecycle phase")
	}
	if e := r.Policy.Validate(); e != nil {
		return nil, e
	}
	if e := r.Selection.Validate(); e != nil {
		return nil, e
	}
	if r.Session != "" && (filepath.Base(r.Session) == r.Session || !filepath.IsAbs(r.Session)) {
		return nil, domain.E(domain.Invalid, "invalid mail runtime path")
	}
	return &r, nil
}
func (s MailService) Status() (MailStatus, error) {
	var out MailStatus
	b, e := s.Store.ReadWorkload("mail.json")
	if e != nil {
		return out, e
	}
	out.Record, e = mailRecord(b)
	if e != nil {
		return out, e
	}
	opBytes, err := s.Store.ReadWorkload("mail-operation.json")
	if err != nil {
		return out, err
	}
	out.LastOperation, err = readMailOperation(opBytes)
	if err != nil {
		return out, err
	}
	if out.LastOperation != nil && out.LastOperation.Phase != "complete" {
		out.RecoveryRequired = true
	}
	out.Disk, e = s.Store.MailDisk()
	if e != nil {
		return out, e
	}
	if out.Record != nil {
		out.Alive, e = process.CheckedAlive(out.Record.Process)
		if e != nil {
			return out, e
		}
		out.RecoveryRequired = out.RecoveryRequired || out.Record.Phase == "applying" || (out.Record.Phase == "running" && !out.Alive)
		out.Available = out.Record.Phase == "running" && out.Alive
	}
	if !out.Available {
		out.Reason = "Owned mail capture is stopped or unavailable; reads never start it."
	}
	return out, nil
}
func (s MailService) client() (*mailpit.Client, *MailRecord, error) {
	st, e := s.Status()
	if e != nil {
		return nil, nil, e
	}
	if !st.Available {
		return nil, st.Record, domain.E(domain.Unavailable, "owned mail capture is unavailable; review mail serve")
	}
	b, e := state.ReadMailCredentials(st.Record.Session)
	if e != nil {
		return nil, nil, domain.E(domain.Unavailable, "owned mail API credentials unavailable")
	}
	var c mailpit.Credentials
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, nil, domain.E(domain.Invalid, "invalid private mail credentials")
	}
	client, e := mailpit.NewClient(st.Record.Policy.APIPort, c)
	return client, st.Record, e
}
func sanitizePage(p mailpit.Page) mailpit.Page {
	for i := range p.Messages {
		m := &p.Messages[i]
		m.Subject = mailpit.Safe(m.Subject)
		m.From.Name = mailpit.Safe(m.From.Name)
		m.From.Address = mailpit.Safe(m.From.Address)
		for j := range m.To {
			m.To[j].Name = mailpit.Safe(m.To[j].Name)
			m.To[j].Address = mailpit.Safe(m.To[j].Address)
		}
	}
	return p
}
func (s MailService) Read(ctx context.Context, req MailRequest) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if req.DryRun || req.Approval != "" {
		return nil, domain.E(domain.Invalid, "mail reads do not accept approval or dry-run")
	}
	if req.Action == "status" {
		return s.Status()
	}
	c, _, e := s.client()
	if e != nil {
		return nil, e
	}
	defer c.Close()
	switch req.Action {
	case "list":
		p, e := c.List(ctx, req.Offset)
		return sanitizePage(p), e
	case "show":
		b, e := c.Raw(ctx, req.ID)
		if e != nil {
			return nil, e
		}
		return mailpit.Parse(req.ID, b)
	}
	return nil, domain.E(domain.Invalid, "unknown mail read action")
}
func (s MailService) ready() error {
	st, e := s.Store.Status()
	if e != nil {
		return e
	}
	if !st.Initialized || st.RecoveryRequired {
		return domain.E(domain.Conflict, "initialize or recover registry state before mail management")
	}
	return nil
}
func (s MailService) plan(ctx context.Context, req MailRequest) (MailPlan, []byte, error) {
	p := MailPlan{Action: req.Action, Effects: []string{}, Limitations: "Current-user foreground loopback capture only. SMTP is unauthenticated locally. Retention deletion and attachment export cannot be undone. Limits are periodic retention targets, not a disk quota. Mailpit authenticated features outside the Velnix endpoint allowlist are not kernel-isolated."}
	if e := s.ready(); e != nil {
		return p, nil, e
	}
	b, e := s.Store.ReadWorkload("mail.json")
	if e != nil {
		return p, nil, e
	}
	r, e := mailRecord(b)
	if e != nil {
		return p, nil, e
	}
	p.Before = hash(b)
	opBytes, err := s.Store.ReadWorkload("mail-operation.json")
	if err != nil {
		return p, nil, err
	}
	op, err := readMailOperation(opBytes)
	if err != nil {
		return p, nil, err
	}
	p.OperationBefore = hash(opBytes)
	if op != nil && op.Phase != "complete" && req.Action != "recover" {
		return p, nil, domain.E(domain.Conflict, "mail operation requires review; stop capture and preview mail recover; evidence is preserved")
	}

	var content []byte
	switch req.Action {
	case "serve":
		if os.Geteuid() == 0 {
			return p, nil, domain.E(domain.Denied, "mail capture must run unprivileged")
		}
		if e = req.Policy.Validate(); e != nil {
			return p, nil, e
		}
		p.Policy = req.Policy
		if r != nil {
			live, e := process.CheckedAlive(r.Process)
			if e != nil {
				return p, nil, e
			}
			if live || r.Phase != "stopped" {
				return p, nil, domain.E(domain.Conflict, "mail session is active or requires reviewed mail recover")
			}
		}
		p.Selection, e = mailpit.Select(req.Executable)
		if e != nil {
			return p, nil, e
		}
		fd, e := process.Probe()
		if e != nil {
			return p, nil, domain.E(domain.Unavailable, "mail lifecycle requires Linux pidfds")
		}
		unix.Close(fd)
		p.Disk, e = s.Store.MailDisk()
		if e != nil {
			return p, nil, e
		}
		p.Runtime, e = state.MailRuntimeBase()
		if e != nil {
			return p, nil, e
		}
		p.Effects = []string{"create private owned mail storage if absent", "execute exact Mailpit with fixed /bin/sh child umask 077 and a minimal environment", "bind SMTP/API only on 127.0.0.1; use private random API credentials", "capture incoming messages; periodically delete messages beyond approved count/age, including existing expired messages", "stop this session's child on cancellation; retain persistent inbox"}
	case "recover":
		if r == nil {
			if op == nil {
				return p, nil, domain.E(domain.Unavailable, "no mail lifecycle or operation record to recover")
			}
			p.Effects = []string{"archive interrupted mail operation evidence; acknowledge it without replaying effects or deleting data"}
			break
		}
		p.Policy = r.Policy
		live, e := process.CheckedAlive(r.Process)
		if e != nil {
			return p, nil, e
		}
		if live {
			return p, nil, domain.E(domain.Conflict, "mail process is still active; stop its foreground session first")
		}
		parentLive, e := process.CheckedAlive(r.Parent)
		if e != nil {
			return p, nil, e
		}
		if parentLive && r.Phase != "stopped" {
			return p, nil, domain.E(domain.Conflict, "mail session owner is still active; wait for its cleanup")
		}
		p.Effects = []string{"mark exited owned mail session stopped without relaunch or signaling unknown processes", "remove expired session credentials; preserve database and operation evidence"}
	case "export", "prune":
		c, record, e := s.client()
		if e != nil {
			return p, nil, e
		}
		defer c.Close()
		p.Policy = record.Policy
		if req.Action == "export" {
			raw, e := c.Raw(ctx, req.ID)
			if e != nil {
				return p, nil, e
			}
			message, e := mailpit.Parse(req.ID, raw)
			if e != nil {
				return p, nil, e
			}
			a, e := message.Attachment(req.Attachment)
			if e != nil {
				return p, nil, e
			}
			target, e := state.PlanMailExport(req.Output)
			if e != nil {
				return p, nil, e
			}
			p.Export = &target
			p.MessageID = req.ID
			p.Attachment = req.Attachment
			content = a.Bytes
			p.ContentDigest = hash(content)
			p.Effects = []string{fmt.Sprintf("export exactly %d attachment bytes as a new 0600 file at the reviewed path; never open or execute it", len(content))}
		} else {
			all, e := c.All(ctx)
			if e != nil {
				return p, nil, e
			}
			p.Snapshot = hash(data(all))
			p.Candidates = pruneCandidates(all, record.Policy, time.Now())
			p.Effects = []string{fmt.Sprintf("irreversibly delete only %d exact expired/excess message IDs after revalidation; concurrent automatic retention may already remove them", len(p.Candidates))}
		}
	default:
		return p, nil, domain.E(domain.Invalid, "unknown mail management action")
	}
	p.ID = hash(data(p))
	return p, content, nil
}
func pruneCandidates(all []mailpit.Summary, p mailpit.Policy, now time.Time) []string {
	sort.Slice(all, func(i, j int) bool {
		if all[i].Created.Equal(all[j].Created) {
			return all[i].ID < all[j].ID
		}
		return all[i].Created.After(all[j].Created)
	})
	out := []string{}
	cutoff := now.Add(-time.Duration(p.MaxDays) * 24 * time.Hour)
	for i, m := range all {
		if i >= p.MaxMessages || m.Created.Before(cutoff) {
			out = append(out, m.ID)
		}
	}
	sort.Strings(out)
	return out
}
func (s MailService) Plan(ctx context.Context, req MailRequest) (MailPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if req.DryRun && req.Approval != "" {
		return MailPlan{}, domain.E(domain.Invalid, "dry-run cannot be combined with approval")
	}
	var p MailPlan
	e := s.Store.WithHTTP(ctx, func(state.HTTPFiles) error { var e error; p, _, e = s.plan(ctx, req); return e })
	return p, e
}
func (s MailService) Apply(ctx context.Context, req MailRequest) (any, error) {
	if req.Approval == "" || req.DryRun {
		return nil, domain.E(domain.Invalid, "mail apply requires an approved plan without dry-run")
	}
	if req.Action == "serve" {
		return s.serve(ctx, req)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result any
	e := s.Store.WithHTTP(ctx, func(f state.HTTPFiles) error {
		p, content, e := s.plan(ctx, req)
		if e != nil {
			return e
		}
		if p.ID != req.Approval {
			return domain.E(domain.Conflict, "mail plan changed; review a fresh plan")
		}
		if req.Action == "recover" {
			previous, err := f.Read("mail-operation.json")
			if err != nil {
				return err
			}
			if previous != nil {
				if err = f.Write("mail-recovery-evidence.json", previous); err != nil {
					return err
				}
			}
		}
		op := MailOperation{Schema: 1, Plan: p, Phase: "applying"}
		if e = f.Write("mail-operation.json", data(op)); e != nil {
			return e
		}
		switch req.Action {
		case "export":
			e = state.WriteMailExport(*p.Export, content)
			if e == nil {
				op.Completed = 1
			}
		case "prune":
			var c *mailpit.Client
			c, _, e = s.client()
			if e == nil {
				defer c.Close()
				e = c.Delete(ctx, p.Candidates)
				if e == nil {
					var all []mailpit.Summary
					all, e = c.All(ctx)
					if e == nil {
						remaining := map[string]bool{}
						for _, m := range all {
							remaining[m.ID] = true
						}
						for _, id := range p.Candidates {
							if remaining[id] {
								e = domain.E(domain.Conflict, "mail prune incomplete; inspect preserved operation evidence")
							} else {
								op.Completed++
							}
						}
					}
				}
			}
		case "recover":
			var b []byte
			b, e = f.Read("mail.json")
			if e == nil {
				var r *MailRecord
				r, e = mailRecord(b)
				if e == nil && r != nil {
					e = state.RemoveMailCredentials(r.Session)
					if e == nil {
						r.Phase = "stopped"
						e = f.Write("mail.json", data(r))
						op.Completed = 1
					}
				}
			}
		}
		if e != nil {
			op.Phase = "recovery-required"
			writeErr := f.Write("mail-operation.json", data(op))
			return errors.Join(e, writeErr)
		}
		op.Phase = "complete"
		if e = f.Write("mail-operation.json", data(op)); e != nil {
			return e
		}
		result = op
		return nil
	})
	return result, e
}
func (s MailService) serve(ctx context.Context, req MailRequest) (any, error) {
	lease, e := s.Store.MailLease()
	if e != nil {
		return nil, e
	}
	defer lease.Close()
	var record MailRecord
	var cmdDone chan error
	var child *os.Process
	var session string
	var providerFile *os.File
	var disk *state.MailData
	// Every launch failure stops only the child created by this call. Pdeathsig
	// also kills that child if the owner crashes before recording its identity.
	cleanupChild := func() {
		if child != nil {
			_ = child.Kill()
			if cmdDone != nil {
				<-cmdDone
				cmdDone = nil
			}
		}
	}
	defer func() {
		cleanupChild()
		if providerFile != nil {
			providerFile.Close()
		}
		if disk != nil {
			disk.Close()
		}
		if session != "" {
			_ = state.RemoveMailCredentials(session)
		}
	}()
	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel()
	e = s.Store.WithHTTP(startupCtx, func(f state.HTTPFiles) error {
		p, _, e := s.plan(ctx, req)
		if e != nil {
			return e
		}
		if p.ID != req.Approval {
			return domain.E(domain.Conflict, "mail plan changed; review a fresh plan")
		}
		// Check conflicts without touching any existing listener. A later bind race
		// still fails provider readiness; no takeover is attempted.
		for _, port := range []int{p.Policy.SMTPPort, p.Policy.APIPort} {
			l, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
			if e != nil {
				return domain.E(domain.Conflict, "mail loopback port %d is unavailable", port)
			}
			l.Close()
		}
		providerFile, e = nativeexec.Open(p.Selection)
		if e != nil {
			return e
		}
		op := MailOperation{Schema: 1, Plan: p, Phase: "applying"}
		if e = f.Write("mail-operation.json", data(op)); e != nil {
			return e
		}
		disk, e = s.Store.MailData(p.Disk.Inode == 0)
		if e != nil {
			return e
		}
		token := make([]byte, 32)
		if _, e = rand.Read(token); e != nil {
			return e
		}
		credentials := mailpit.Credentials{User: "velnix", Password: hex.EncodeToString(token)}
		sessionID := make([]byte, 16)
		if _, e = rand.Read(sessionID); e != nil {
			return e
		}
		session, e = state.CreateMailSession(p.Runtime, "mail-"+hex.EncodeToString(sessionID), data(credentials))
		if e != nil {
			return e
		}
		parent, e := process.Observe(os.Getpid())
		if e != nil {
			return e
		}
		record = MailRecord{Schema: 1, Phase: "applying", PlanID: p.ID, Selection: p.Selection, Policy: p.Policy, Parent: parent, Session: session}
		if e = f.Write("mail.json", data(record)); e != nil {
			return e
		}
		cmd := mailpit.Command(providerFile, disk.File(), p.Selection, p.Policy, credentials, session)
		if e = cmd.Start(); e != nil {
			return domain.E(domain.Unavailable, "mail child launch failed; inspect mail recover")
		}
		child = cmd.Process
		cmdDone = make(chan error, 1)
		go func() { cmdDone <- cmd.Wait() }()
		client, e := mailpit.NewClient(p.Policy.APIPort, credentials)
		if e != nil {
			return e
		}
		defer client.Close()
		startup, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		for {
			id, err := process.Observe(child.Pid)
			if err == nil && id.Executable == p.Selection.Path {
				record.Process = id
				// Persist identity before querying readiness so startup interruption is
				// recoverable even when the provider never becomes healthy.
				if e = f.Write("mail.json", data(record)); e != nil {
					return e
				}
				if _, err = client.List(startup, 0); err == nil {
					break
				}
			}
			select {
			case <-startup.Done():
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return domain.E(domain.Unavailable, "mail startup failed or timed out; inbox preserved; review mail status/recover")
			case e := <-cmdDone:
				cmdDone = nil
				child = nil
				return domain.E(domain.Unavailable, "mail provider exited during startup (%v); inbox preserved", e)
			case <-time.After(40 * time.Millisecond):
			}
		}
		if _, e = disk.Snapshot(); e != nil {
			return e
		}
		record.Phase = "running"
		if e = f.Write("mail.json", data(record)); e != nil {
			return e
		}
		op.Phase = "complete"
		op.Completed = 1
		return f.Write("mail-operation.json", data(op))
	})
	if e != nil {
		return nil, e
	}
	// The global mutation lock is released for the lifetime of the foreground child.
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	if s.Ready != nil {
		if readyErr := s.Ready(record); readyErr != nil {
			e = readyErr
			stop()
		}
	}
	select {
	case <-runCtx.Done():
		_ = process.Signal(record.Process, unix.SIGTERM)
		select {
		case <-cmdDone:
			cmdDone = nil
			child = nil
		case <-time.After(3 * time.Second):
			cleanupChild()
		}
	case err := <-cmdDone:
		cmdDone = nil
		child = nil
		if err != nil {
			e = domain.E(domain.Unavailable, "mail provider exited; inbox preserved")
		}
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finishErr := s.Store.WithHTTP(cleanupCtx, func(f state.HTTPFiles) error {
		b, err := f.Read("mail.json")
		if err != nil {
			return err
		}
		current, err := mailRecord(b)
		if err != nil {
			return err
		}
		if current == nil || current.PlanID != record.PlanID || current.Session != record.Session {
			return domain.E(domain.Conflict, "mail lifecycle evidence changed; refusing cleanup overwrite")
		}
		if err = state.RemoveMailCredentials(session); err != nil {
			return err
		}
		record.Phase = "stopped"
		return f.Write("mail.json", data(record))
	})
	if e == nil && ctx.Err() != nil {
		e = ctx.Err()
	}
	return record, errors.Join(e, finishErr)
}
