package app

import (
	"context"

	"velnix/internal/config"
	"velnix/internal/platform"
	"velnix/internal/state"
)

type PlatformReader interface {
	Detect(context.Context) (platform.Facts, error)
}
type StateReader interface{ Status() (state.Status, error) }
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type DoctorReport struct {
	Platform platform.Facts `json:"platform"`
	Paths    platform.Paths `json:"paths"`
	Checks   []Check        `json:"checks"`
	Healthy  bool           `json:"healthy"`
}
type Doctor struct {
	Platform PlatformReader
	State    StateReader
	Paths    platform.Paths
}

func (d Doctor) Run(ctx context.Context, cfg config.Result) (DoctorReport, error) {
	facts, err := d.Platform.Detect(ctx)
	if err != nil {
		return DoctorReport{}, err
	}
	report := DoctorReport{Platform: facts, Paths: d.Paths, Healthy: true, Checks: []Check{}}
	add := func(name, status, detail string) {
		report.Checks = append(report.Checks, Check{name, status, detail})
		if status == "error" {
			report.Healthy = false
		}
	}
	if facts.OS != "linux" {
		add("platform", "error", "Only Linux is supported")
	} else if facts.Distribution != "ubuntu" || facts.DistributionVersion != "24.04" {
		add("platform", "warning", "Outside the Ubuntu 24.04 initial validation target")
	} else {
		add("platform", "ok", "Ubuntu 24.04 initial target; host compatibility still requires validation")
	}
	if cfg.Exists {
		add("config", "ok", "User configuration validated")
	} else {
		add("config", "ok", "No configuration file; using defaults")
	}
	st, err := d.State.Status()
	switch {
	case err != nil:
		add("state", "error", err.Error())
	case st.RecoveryRequired:
		add("state", "error", "Initialization interrupted; preview state recover")
	case st.Initialized:
		add("state", "ok", "Registry schema and initialization journal validated")
	default:
		add("state", "ok", "Not initialized; no state has been created")
	}
	if facts.Systemd {
		add("systemd", "ok", "System manager metadata detected; lifecycle and user manager not probed")
	} else {
		add("systemd", "warning", "No system manager detected; daemonless diagnostics remain available")
	}
	add("providers", "info", "No runtime binaries, package managers, services, project hooks or Windows commands executed")
	return report, nil
}
