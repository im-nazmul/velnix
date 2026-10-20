package nginx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"velnix/internal/domain"
)

// Quote rejects all syntax-bearing characters rather than accepting config
// fragments. Nginx variable expansion is forbidden even inside quoted paths.
func quote(s string) (string, error) {
	for _, r := range s {
		if r < 32 || r == 127 || strings.ContainsRune("\"\\$;{}", r) {
			return "", domain.E(domain.Invalid, "path contains unsupported Nginx syntax")
		}
	}
	return "\"" + s + "\"", nil
}
func Render(projects []domain.Project, port int, token, prefix string) (string, error) {
	if port < 1024 || port > 65535 {
		return "", domain.E(domain.Invalid, "HTTP port must be 1024–65535")
	}
	q, e := quote(filepath.Join(prefix, "nginx.pid"))
	if e != nil {
		return "", e
	}
	var b strings.Builder
	fmt.Fprintf(&b, "daemon off;\nmaster_process on;\nworker_processes 1;\nworker_shutdown_timeout 2s;\npid %s;\nerror_log stderr warn;\nevents { worker_connections 128; }\nhttp {\naccess_log off;\ndefault_type application/octet-stream;\nclient_max_body_size 1m;\nclient_body_buffer_size 1m;\n", q)
	for _, name := range []string{"client_body_temp_path", "proxy_temp_path", "fastcgi_temp_path", "uwsgi_temp_path", "scgi_temp_path"} {
		p, e := quote(filepath.Join(prefix, name))
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&b, "%s %s;\n", name, p)
	}
	// Embedded MIME map avoids reading distro configuration or modules.
	b.WriteString("types { text/html html htm; text/css css; application/javascript js mjs; application/json json; image/png png; image/jpeg jpg jpeg; image/svg+xml svg; image/x-icon ico; text/plain txt; font/woff2 woff2; }\n")
	fmt.Fprintf(&b, "server { listen 127.0.0.1:%d default_server; server_name _; return 404; }\n", port)
	fmt.Fprintf(&b, "server { listen 127.0.0.1:%d; server_name velnix-health.invalid; location = / { default_type text/plain; return 200 '%s'; } location / { return 404; } }\n", port, token)
	for _, p := range projects {
		if e := p.Validate(); e != nil {
			return "", e
		}
		root, e := quote(p.DocumentRoot)
		if e != nil {
			return "", e
		}
		fmt.Fprintf(&b, "server { listen 127.0.0.1:%d; server_name %s; root %s; disable_symlinks on; autoindex off; index index.html;\n", port, p.Domain, root)
		b.WriteString("location ~ (^|/)\\. { return 404; }\nlocation ~* (^|/)(vendor|node_modules|storage|config|database|tests|uploads)(/|$) { return 404; }\nlocation ~* (\\.(php[0-9]*|phtml|phar|py|pyc|go|rb|pl|sh|sql|sqlite[0-9]*|log|pem|key|crt|bak|old|orig|swp|ini|toml|ya?ml|lock)|~)$ { return 404; }\nlocation ~* (^|/)(composer\\.json|package\\.json|artisan|Dockerfile|Makefile)$ { return 404; }\nlocation / { limit_except GET HEAD { deny all; } try_files $uri $uri/ =404; }\n}\n")
	}
	b.WriteString("}\n")
	return b.String(), nil
}

// Tail bounds retained diagnostics while always draining child output.
type Tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *Tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	t.b = append(t.b, p...)
	if len(t.b) > 8192 {
		t.b = append([]byte(nil), t.b[len(t.b)-8192:]...)
	}
	return n, nil
}
func (t *Tail) String() string { t.mu.Lock(); defer t.mu.Unlock(); return string(bytes.TrimSpace(t.b)) }
func Command(exe, prefix, config string) *exec.Cmd {
	c := exec.Command(exe, "-p", prefix+"/", "-c", config)
	c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	c.Dir = "/"
	// Workers inherit diagnostics pipes. A killed master must not leave Wait
	// blocked on those pipes before the owner can clean up recorded workers.
	c.WaitDelay = time.Second
	return c
}
func Validate(ctx context.Context, exe, prefix, config string) error {
	c := Command(exe, prefix, config)
	c.Args = append(c.Args, "-t")
	var out Tail
	c.Stdout = &out
	c.Stderr = &out
	if e := c.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case e := <-done:
		if e != nil {
			return domain.E(domain.Unavailable, "Nginx validation failed: %s", out.String())
		}
		return nil
	case <-ctx.Done():
		_ = c.Process.Kill()
		<-done
		return ctx.Err()
	}
}
func Executable(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", domain.E(domain.Invalid, "Nginx executable must be absolute")
	}
	p, e := filepath.EvalSymlinks(path)
	if e != nil {
		return "", domain.E(domain.Unavailable, "Nginx executable unavailable: %v", e)
	}
	s, e := os.Stat(p)
	if e != nil {
		return "", e
	}
	if !s.Mode().IsRegular() || s.Mode()&0111 == 0 {
		return "", domain.E(domain.Unavailable, "Nginx executable is not executable")
	}
	return p, nil
}
