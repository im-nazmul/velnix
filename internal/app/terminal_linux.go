package app

import (
	"context"
	"encoding/json"
	"fmt"

	"velnix/internal/config"
	"velnix/internal/domain"
	"velnix/internal/platform"
	"velnix/internal/state"
)

// TerminalEntry is presentation-neutral read-only inspection data.
type TerminalEntry struct{ Name, Detail string }
type TerminalQuery struct {
	Paths    platform.Paths
	Platform PlatformReader
}

func terminalEntry(name string, value any, err error) TerminalEntry {
	if err != nil {
		return TerminalEntry{name, "Unavailable: " + err.Error()}
	}
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return TerminalEntry{name, e.Error()}
	}
	return TerminalEntry{name, string(b)}
}
func (q TerminalQuery) Read(ctx context.Context, screen int) []TerminalEntry {
	store := state.Store{Paths: q.Paths}
	entries := []TerminalEntry{}
	add := func(name string, v any, e error) { entries = append(entries, terminalEntry(name, v, e)) }
	if ctx.Err() != nil {
		return entries
	}
	projects, err := (ProjectService{Store: store}).Run(ctx, ProjectRequest{ProjectChange: state.ProjectChange{Action: "list"}})
	switch screen {
	case 0:
		if err != nil {
			add("Projects", nil, err)
		} else {
			add("Registered projects", len(projects.([]domain.Project)), nil)
		}
		v, e := (HTTPService{Store: store}).Status()
		add("HTTP", v, e)
		t, e := (TLSService{Store: store}).Status()
		add("TLS", t, e)
		if err != nil {
			add("FPM registry", nil, err)
		} else {
			for _, p := range projects.([]domain.Project) {
				if ctx.Err() != nil {
					break
				}
				f, e := (HTTPService{Store: store, FPMName: p.Name}).Status()
				add("FPM "+p.Name, f, e)
			}
		}
		entries = append(entries, q.diagnostics(ctx)...)
	case 1:
		if err != nil {
			add("Projects", nil, err)
			break
		}
		for _, p := range projects.([]domain.Project) {
			if ctx.Err() != nil {
				break
			}
			b, _ := json.MarshalIndent(p, "", "  ")
			v, e := (ProjectService{Store: store}).Run(ctx, ProjectRequest{ProjectChange: state.ProjectChange{Action: "detect", Name: p.Name}})
			meta := terminalEntry("Metadata", v, e)
			f, e := (HTTPService{Store: store, FPMName: p.Name}).Status()
			web := terminalEntry("Web/FPM profile", f, e)
			entries = append(entries, TerminalEntry{p.Name, string(b) + "\n\nMetadata\n" + meta.Detail + "\n\nWeb/FPM profile\n" + web.Detail})
		}
	case 2:
		add("Provider capabilities", RuntimeProviders(), nil)
		for _, kind := range []string{"php", "node", "bun", "python"} {
			if ctx.Err() != nil {
				break
			}
			v, e := (RuntimeService{Store: store}).Run(ctx, RuntimeRequest{Action: "list", Kind: kind})
			add(kind+" candidates (not probed)", v, e)
		}
		if err != nil {
			add("Project selections", nil, err)
		} else {
			for _, p := range projects.([]domain.Project) {
				add(p.Name+" selections (stored evidence)", struct {
					PHP               domain.PHPSelection
					Node, Bun, Python domain.RuntimeSelection
				}{p.PHPCLI, p.Node, p.Bun, p.Python}, nil)
			}
		}
	case 3:
		v, e := (HTTPService{Store: store}).Status()
		add("Owned HTTP", v, e)
		t, e := (TLSService{Store: store}).Status()
		add("Owned TLS", t, e)
		if err != nil {
			add("FPM registry", nil, err)
		} else {
			for _, p := range projects.([]domain.Project) {
				if ctx.Err() != nil {
					break
				}
				v, e := (HTTPService{Store: store, FPMName: p.Name}).Status()
				add("FPM "+p.Name, v, e)
			}
		}
		entries = append(entries, TerminalEntry{"System databases", "Unsupported in this slice: PostgreSQL, MariaDB/MySQL, Redis. No system service commands are executed."})
	case 4:
		entries = q.diagnostics(ctx)
	}
	if len(entries) == 0 {
		entries = append(entries, TerminalEntry{"No registered projects", "Use velnix projects link --help for the existing reviewed CLI workflow."})
	}
	return entries
}
func (q TerminalQuery) diagnostics(ctx context.Context) []TerminalEntry {
	cfg, e := config.Load(q.Paths.Config+"/config.toml", false)
	configErr := e
	report, e := (Doctor{Platform: q.Platform, State: state.Store{Paths: q.Paths}, Paths: q.Paths}).Run(ctx, cfg)
	entries := []TerminalEntry{terminalEntry("Platform and paths", report.Platform, e)}
	if configErr != nil {
		entries = append(entries, terminalEntry("Configuration", nil, configErr))
	}
	for _, c := range report.Checks {
		if configErr != nil && c.Name == "config" {
			continue
		}
		entries = append(entries, TerminalEntry{c.Name, fmt.Sprintf("%s: %s", c.Status, c.Detail)})
	}
	return append(entries, TerminalEntry{"CLI next steps", "velnix doctor\nvelnix config validate\nvelnix state status\nReview CLI plans before any mutation; no automatic repair is performed here."})
}
