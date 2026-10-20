//go:build linux

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/runtimeinventory"
)

type RuntimeProbePlan struct {
	Action      string                     `json:"action"`
	Kind        string                     `json:"kind"`
	Executable  domain.ExecutableSelection `json:"executable"`
	Argv        []string                   `json:"argv"`
	Directory   string                     `json:"directory"`
	Environment []string                   `json:"environment"`
	Limitations string                     `json:"limitations"`
	ID          string                     `json:"id"`
}

type RuntimeInspection struct {
	Kind       string                     `json:"kind"`
	Executable domain.ExecutableSelection `json:"executable"`
	Origin     string                     `json:"origin"`
	Authority  string                     `json:"authority"`
	Probed     bool                       `json:"probed"`
}

func inspectRuntime(ctx context.Context, req RuntimeRequest) (any, error) {
	args, err := runtimeinventory.ProbeArgs(req.Kind)
	if err != nil {
		return nil, err
	}
	s, err := nativeexec.Snapshot(req.Path, req.Version)
	if err != nil {
		return nil, err
	}
	p := RuntimeProbePlan{
		Action: "probe-runtime-version", Kind: req.Kind, Executable: s, Argv: args,
		Directory: "/", Environment: []string{"PATH=/usr/bin:/bin", "LANG=C"},
		Limitations: "Runs the explicitly selected external native executable, with no inherited environment, stdin or project cwd. Version output is not proof of runtime identity or a sandbox. No installation, ownership or persistent state changes. Timeout: 2 seconds; retained stdout/stderr: 4 KiB each.",
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(b)
	p.ID = hex.EncodeToString(h[:])
	if req.Approval == "" {
		return p, nil
	}
	if req.Approval != p.ID {
		return nil, domain.E(domain.Conflict, "runtime probe plan changed; preview again")
	}
	if err := runtimeinventory.Probe(ctx, req.Kind, s); err != nil {
		return nil, err
	}
	return RuntimeInspection{Kind: req.Kind, Executable: s, Origin: "unknown", Authority: "external", Probed: true}, nil
}
