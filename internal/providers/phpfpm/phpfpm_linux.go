//go:build linux

// Package phpfpm renders isolated, user-owned pools without distro includes.
package phpfpm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/nginx"
)

type Config struct {
	Project   domain.Project             `json:"project"`
	Selection domain.ExecutableSelection `json:"selection"`
	Profile   string                     `json:"profile"`
	Uploads   []string                   `json:"uploads"`
	Socket    string                     `json:"socket"`
}

func Quote(s string) (string, error) {
	for _, r := range s {
		if r < 32 || r == 127 || strings.ContainsRune("\"\\$;{}[]", r) {
			return "", domain.E(domain.Invalid, "unsupported FPM path syntax")
		}
	}
	return "\"" + s + "\"", nil
}
func (c Config) Validate() error {
	if e := c.Project.Validate(); e != nil {
		return e
	}
	if e := c.Selection.Validate(); e != nil {
		return e
	}
	if c.Profile != "php" && c.Profile != "front-controller" {
		return domain.E(domain.Invalid, "profile must be php or front-controller")
	}
	if !filepath.IsAbs(c.Socket) || len(c.Socket) > 100 {
		return domain.E(domain.Invalid, "private FPM socket path must be absolute and at most 100 bytes; use a shorter XDG state path")
	}
	for _, p := range append([]string{c.Socket, c.Project.Root, c.Project.DocumentRoot}, c.Uploads...) {
		if _, e := Quote(p); e != nil {
			return e
		}
	}
	for _, p := range c.Uploads {
		if p == "" || filepath.IsAbs(p) || filepath.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") || !regexp.MustCompile(`^[a-zA-Z0-9_/-]+$`).MatchString(p) {
			return domain.E(domain.Invalid, "upload paths must be clean relative directories with letters, digits, slash, underscore or hyphen")
		}
	}
	return nil
}
func Render(c Config, prefix, name string) (string, error) {
	if e := c.Validate(); e != nil {
		return "", e
	}
	q := func(p string) string { v, _ := Quote(p); return v }
	for _, p := range []string{prefix, name} {
		if _, e := Quote(p); e != nil {
			return "", e
		}
	}
	return fmt.Sprintf("[global]\ndaemonize = no\nerror_log = /proc/self/fd/2\npid = %s\n[velnix]\nlisten = %s\nlisten.mode = 0600\npm = static\npm.max_children = 1\nchdir = %s\nclear_env = yes\ncatch_workers_output = yes\ndecorate_workers_output = no\nsecurity.limit_extensions = .php\nrequest_terminate_timeout = 30s\nphp_admin_flag[log_errors] = on\nphp_admin_value[error_log] = /proc/self/fd/2\nphp_admin_value[cgi.fix_pathinfo] = 0\n", q(filepath.Join(prefix, name+".pid")), q(c.Socket), q(c.Project.Root)), nil
}
func Command(f *os.File, c Config, config string) *exec.Cmd {
	cmd := nativeexec.Command(f, c.Selection, "-y", config, "-F")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	cmd.Dir = "/"
	return cmd
}
func Version(ctx context.Context, c Config) error {
	f, e := nativeexec.Open(c.Selection)
	if e != nil {
		return e
	}
	defer f.Close()
	cmd := nativeexec.Command(f, c.Selection, "-n", "-v")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "PHP_INI_SCAN_DIR="}
	cmd.Dir = "/"
	var out nginx.Tail
	cmd.Stdout = &out
	cmd.Stderr = &out
	if e = run(ctx, cmd); e != nil {
		return e
	}
	// FPM's version banner is distinct from CLI; never accept another SAPI.
	if !strings.HasPrefix(out.String(), "PHP "+c.Selection.Version+" (fpm-fcgi)") {
		return domain.E(domain.Unavailable, "FPM exact version/SAPI mismatch: %s", out.String())
	}
	return nil
}
func Validate(ctx context.Context, c Config, path string) error {
	f, e := nativeexec.Open(c.Selection)
	if e != nil {
		return e
	}
	defer f.Close()
	cmd := Command(f, c, path)
	cmd.Args = append(cmd.Args, "-t")
	var tail nginx.Tail
	cmd.Stdout = &tail
	cmd.Stderr = &tail
	if e = run(ctx, cmd); e != nil {
		return domain.E(domain.Unavailable, "FPM validation failed: %v: %s", e, tail.String())
	}
	return nil
}
func run(ctx context.Context, cmd *exec.Cmd) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case e := <-done:
		return e
	case <-ctx.Done():
		cmd.Process.Kill()
		<-done
		return ctx.Err()
	case <-timer.C:
		cmd.Process.Kill()
		<-done
		return domain.E(domain.Unavailable, "FPM probe timed out")
	}
}
