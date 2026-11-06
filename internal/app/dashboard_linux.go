//go:build linux

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"velnix/internal/config"
	"velnix/internal/domain"
	"velnix/internal/platform"
	"velnix/internal/providers/mailpit"
	"velnix/internal/state"
)

type DashboardLaunch struct {
	ID             string          `json:"id"`
	Action         string          `json:"action"`
	Address        string          `json:"address"`
	Paths          platform.Paths  `json:"paths"`
	Runtime        state.UIRuntime `json:"runtime"`
	CredentialFile string          `json:"credential_file_template"`
	Effects        []string        `json:"effects"`
	Limitations    string          `json:"limitations"`
}

func PlanDashboard(paths platform.Paths, port int) (DashboardLaunch, error) {
	p := DashboardLaunch{Action: "serve-dashboard", Paths: paths}
	if os.Geteuid() == 0 {
		return p, domain.E(domain.Denied, "dashboard must run as an unprivileged user")
	}
	if port < 1024 || port > 65535 {
		return p, domain.E(domain.Invalid, "dashboard port must be 1024..65535")
	}
	var e error
	p.Runtime, e = state.DashboardRuntime()
	if e != nil {
		return p, e
	}
	p.Address = fmt.Sprintf("127.0.0.1:%d", port)
	p.CredentialFile = filepath.Join(p.Runtime.Base, "ui-SESSION", "bootstrap")
	p.Effects = []string{"create a private runtime directory and one-time bootstrap file (0700/0600)", "serve embedded local dashboard and authenticated API on the exact loopback address", "revoke browser sessions, remove this launch's bootstrap file and close listener on shutdown"}
	p.Limitations = "Foreground only. No browser is opened. Reads never initialize state or probe runtimes. Each mutation needs a separate exact plan approval. Plain loopback HTTP is not encrypted; same-UID code is not isolated. Bootstrap expires after five minutes; idle/absolute session expiry requires restart. Abandoned runtime directories are not adopted or automatically deleted."
	p.ID = hash(data(p))
	return p, nil
}

type Dashboard struct {
	Paths    platform.Paths
	Platform PlatformReader
}
type DashboardIssue struct {
	Area    string `json:"area"`
	Message string `json:"message"`
}
type DashboardState struct {
	Initialized      bool `json:"initialized"`
	RecoveryRequired bool `json:"recovery_required"`
}
type DashboardWorkload struct {
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	Phase            string `json:"phase"`
	Alive            bool   `json:"alive"`
	RecoveryRequired bool   `json:"recovery_required"`
	Port             int    `json:"port,omitempty"`
	Reason           string `json:"reason,omitempty"`
}
type DashboardCertificate struct {
	Phase       string    `json:"phase"`
	Names       []string  `json:"names"`
	Expires     time.Time `json:"expires"`
	Fingerprint string    `json:"fingerprint"`
}
type DashboardInventory struct {
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}
type DashboardData struct {
	State       *DashboardState       `json:"state,omitempty"`
	Projects    []domain.Project      `json:"projects,omitempty"`
	Workloads   []DashboardWorkload   `json:"workloads,omitempty"`
	Runtimes    []DashboardInventory  `json:"runtimes,omitempty"`
	Certificate *DashboardCertificate `json:"certificate,omitempty"`
	Checks      []Check               `json:"checks,omitempty"`
	Platform    *platform.Facts       `json:"platform,omitempty"`
	Issues      []DashboardIssue      `json:"issues"`
	Truncated   bool                  `json:"truncated"`
}

func (d Dashboard) Read(ctx context.Context, page string) (DashboardData, error) {
	out := DashboardData{Issues: []DashboardIssue{}}
	store := state.Store{Paths: d.Paths}
	switch page {
	case "overview", "projects", "runtimes", "services", "certificates", "mail", "diagnostics":
	default:
		return out, domain.E(domain.Invalid, "unknown dashboard page")
	}
	issue := func(area string, e error) {
		if e != nil {
			out.Issues = append(out.Issues, DashboardIssue{area, mailpit.Safe(e.Error())})
		}
	}
	st, e := store.Status()
	if e != nil {
		issue("state", e)
	} else {
		out.State = &DashboardState{st.Initialized, st.RecoveryRequired}
	}
	projects := st.Registry.Projects
	if len(projects) > 200 {
		projects = projects[:200]
		out.Truncated = true
	}
	if page == "overview" || page == "projects" || page == "runtimes" {
		out.Projects = projects
	}
	httpWorkload := func(name string) {
		v, e := (HTTPService{Store: store, FPMName: name}).Status()
		if e != nil {
			issue("web", e)
			return
		}
		kind := "HTTP"
		if name != "" {
			kind = "FPM"
		}
		phase := "stopped"
		port := 0
		if v.Record != nil {
			phase = v.Record.Phase
			port = v.Record.Port
		}
		out.Workloads = append(out.Workloads, DashboardWorkload{Kind: kind, Name: name, Phase: phase, Alive: v.Alive, RecoveryRequired: v.RecoveryRequired, Port: port})
	}
	if page == "overview" || page == "services" {
		httpWorkload("")
		if page == "services" {
			for _, p := range projects {
				if ctx.Err() != nil {
					return out, ctx.Err()
				}
				httpWorkload(p.Name)
			}
		}
	}
	if page == "overview" || page == "services" || page == "mail" {
		m, e := (MailService{Store: store}).Status()
		if e != nil {
			issue("mail", e)
		} else {
			phase := "stopped"
			port := 0
			if m.Record != nil {
				phase = m.Record.Phase
				port = m.Record.Policy.SMTPPort
			}
			out.Workloads = append(out.Workloads, DashboardWorkload{Kind: "Mail", Phase: phase, Alive: m.Available, RecoveryRequired: m.RecoveryRequired, Port: port, Reason: m.Reason})
		}
	}
	if page == "overview" || page == "certificates" {
		c, e := (TLSService{Store: store}).Status()
		issue("certificates", e)
		if e == nil && c != nil {
			out.Certificate = &DashboardCertificate{c.Phase, c.Names, c.Expires, c.Fingerprint}
		}
	}
	if page == "runtimes" {
		for _, kind := range []string{"php", "node", "bun", "python"} {
			v, e := (RuntimeService{Store: store}).Run(ctx, RuntimeRequest{Action: "list", Kind: kind})
			issue(kind, e)
			if e == nil {
				out.Runtimes = append(out.Runtimes, DashboardInventory{kind, v})
			}
		}
	}
	if page == "overview" || page == "diagnostics" {
		cfg, err := config.Load(filepath.Join(d.Paths.Config, "config.toml"), false)
		if err != nil {
			issue("configuration", domain.E(domain.Invalid, "Configuration is unavailable or invalid; inspect velnix config validate."))
		}
		report, e := (Doctor{Platform: d.Platform, State: store, Paths: d.Paths}).Run(ctx, cfg)
		issue("diagnostics", e)
		if e == nil {
			out.Checks = report.Checks
			out.Platform = &report.Platform
		}
	}
	return out, ctx.Err()
}

type DashboardChange struct {
	Operation    string `json:"operation"`
	Name         string `json:"name,omitempty"`
	Root         string `json:"root,omitempty"`
	DocumentRoot string `json:"document_root,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Executable   string `json:"executable,omitempty"`
	Version      string `json:"version,omitempty"`
}
type DashboardReview struct {
	ID     string          `json:"id"`
	Change DashboardChange `json:"change"`
	Detail any             `json:"detail"`
	Notes  []string        `json:"notes"`
}

func (d Dashboard) change(ctx context.Context, c DashboardChange, id string) (any, error) {
	store := state.Store{Paths: d.Paths}
	allowed := DashboardChange{Operation: c.Operation}
	switch c.Operation {
	case "initialize", "prune-mail":
	case "link":
		allowed.Name = c.Name
		allowed.Root = c.Root
		allowed.DocumentRoot = c.DocumentRoot
	case "unlink":
		allowed.Name = c.Name
	case "select-runtime":
		allowed.Name = c.Name
		allowed.Kind = c.Kind
		allowed.Executable = c.Executable
		allowed.Version = c.Version
	case "clear-runtime":
		allowed.Name = c.Name
		allowed.Kind = c.Kind
	default:
		return nil, domain.E(domain.Invalid, "operation is outside dashboard scope")
	}
	if allowed != c {
		return nil, domain.E(domain.Invalid, "unexpected fields for dashboard operation")
	}
	switch c.Operation {
	case "initialize":
		return (StateService{Store: store}).Run(ctx, StateRequest{Action: "init", ApprovedPlanID: id})
	case "link", "unlink":
		if c.Operation == "link" && !filepath.IsAbs(c.Root) {
			return nil, domain.E(domain.Invalid, "link requires an explicit absolute project path")
		}
		root := c.DocumentRoot
		if root == "" {
			root = "."
		}
		return (ProjectService{Store: store}).Run(ctx, ProjectRequest{ProjectChange: state.ProjectChange{Action: c.Operation, Name: c.Name, Root: c.Root, DocumentRoot: root}, ApprovedPlanID: id})
	case "select-runtime", "clear-runtime":
		if c.Kind != "php" && c.Kind != "node" && c.Kind != "bun" && c.Kind != "python" {
			return nil, domain.E(domain.Invalid, "unsupported runtime kind")
		}
		action := "select"
		if c.Operation == "clear-runtime" {
			action = "clear"
		}
		return (RuntimeService{Store: store}).Run(ctx, RuntimeRequest{Action: action, Name: c.Name, Kind: c.Kind, Path: c.Executable, Version: c.Version, Approval: id})
	case "prune-mail":
		s := MailService{Store: store}
		r := MailRequest{Action: "prune", Approval: id}
		if id == "" {
			return s.Plan(ctx, r)
		}
		return s.Apply(ctx, r)
	}
	return nil, domain.E(domain.Invalid, "unsupported dashboard action")
}
func (d Dashboard) Preview(ctx context.Context, c DashboardChange) (DashboardReview, error) {
	p := DashboardReview{Change: c, Notes: []string{"Approval applies only to this exact plan. Changed state requires a fresh review."}}
	v, e := d.change(ctx, c, "")
	if e != nil {
		return p, e
	}
	p.Detail = v
	switch v := v.(type) {
	case StateResult:
		if v.Plan != nil {
			p.ID = v.Plan.ID
		}
	case state.ProjectPlan:
		p.ID = v.ID
	case MailPlan:
		p.ID = v.ID
	}
	if c.Operation == "select-runtime" {
		p.Notes = append(p.Notes, "Apply may execute the exact selected native binary to verify its version. This is not a sandbox. No probe runs during preview or page refresh.")
	}
	if c.Operation == "unlink" {
		p.Notes = append(p.Notes, "Unlink removes registry references, never source files. Active HTTP routing may need a separate reviewed CLI reload.")
	}
	if c.Operation == "prune-mail" {
		p.Notes = append(p.Notes, "Mail deletion is irreversible without a separately verified backup.")
	}
	if p.ID == "" {
		return p, domain.E(domain.Invalid, "backend did not produce a reviewable plan")
	}
	return p, nil
}
func (d Dashboard) Apply(ctx context.Context, c DashboardChange, id string) (any, error) {
	if id == "" {
		return nil, domain.E(domain.Invalid, "exact approval required")
	}
	return d.change(ctx, c, id)
}
func (d Dashboard) MailPage(ctx context.Context, offset int) (mailpit.Page, error) {
	v, e := (MailService{Store: state.Store{Paths: d.Paths}}).Read(ctx, MailRequest{Action: "list", Offset: offset})
	if e != nil {
		return mailpit.Page{}, e
	}
	return v.(mailpit.Page), nil
}
func (d Dashboard) MailMessage(ctx context.Context, id string) (mailpit.Message, error) {
	v, e := (MailService{Store: state.Store{Paths: d.Paths}}).Read(ctx, MailRequest{Action: "show", ID: id})
	if e != nil {
		return mailpit.Message{}, e
	}
	return v.(mailpit.Message), nil
}
func (d Dashboard) Attachment(ctx context.Context, id, part string) (mailpit.Attachment, error) {
	return (MailService{Store: state.Store{Paths: d.Paths}}).Attachment(ctx, id, part)
}
