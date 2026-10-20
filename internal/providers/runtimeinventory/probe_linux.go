//go:build linux

package runtimeinventory

import (
	"bytes"
	"context"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
)

// ProbeArgs only permits version queries, never scripts, package managers or
// repository hooks. Python skips site initialization and inherited Python config.
func ProbeArgs(kind string) ([]string, error) {
	switch kind {
	case "node", "bun":
		return []string{"--version"}, nil
	case "python":
		return []string{"-I", "-S", "--version"}, nil
	default:
		return nil, domain.E(domain.Invalid, "runtime kind must be node, bun or python")
	}
}

type probeOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *probeOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 4096 - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

// Probe requires caller approval. This is a version check, not a sandbox or
// proof that an executable implements a particular language correctly.
func Probe(ctx context.Context, kind string, s domain.ExecutableSelection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args, err := ProbeArgs(kind)
	if err != nil {
		return err
	}
	f, err := nativeexec.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	c := nativeexec.Command(f, s, args...)
	c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	c.Dir = "/"
	var out, stderr probeOutput
	c.Stdout, c.Stderr = &out, &stderr
	if err = c.Start(); err != nil {
		return domain.E(domain.Unavailable, "cannot start runtime probe: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = c.Process.Kill()
		<-done
		return ctx.Err()
	}
	expected := s.Version
	if kind == "node" {
		expected = "v" + expected
	}
	if kind == "python" {
		expected = "Python " + expected
	}
	output := out.String()
	if err != nil || out.overflow || stderr.overflow || stderr.Len() != 0 || (output != expected && output != expected+"\n" && output != expected+"\r\n") {
		return domain.E(domain.Unavailable, "%s version probe did not match required exact version %s or produced unexpected diagnostics", kind, s.Version)
	}
	check, err := nativeexec.Open(s)
	if err != nil {
		return err
	}
	return check.Close()
}
