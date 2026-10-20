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

type PHPRoute struct {
	Socket, Profile string
	Uploads         []string
}

func Render(projects []domain.Project, port int, token, prefix string) (string, error) {
	return RenderWeb(projects, port, token, prefix, nil)
}
func RenderWeb(projects []domain.Project, port int, token, prefix string, routes map[string]PHPRoute) (string, error) {
	if port < 1024 || port > 65535 {
		return "", domain.E(domain.Invalid, "HTTP port must be 1024–65535")
	}
	q, e := quote(filepath.Join(prefix, "nginx.pid"))
	if e != nil {
		return "", e
	}
	var b strings.Builder
	fmt.Fprintf(&b, "daemon off;\nmaster_process on;\nworker_processes 1;\nworker_shutdown_timeout 2s;\npid %s;\nerror_log stderr warn;\nevents { worker_connections 128; }\nhttp {\naccess_log off;\ndefault_type application/octet-stream;\nclient_max_body_size 1m;\nclient_body_buffer_size 1m;\nmap $http_upgrade $velnix_connection { default upgrade; '' close; }\n", q)
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
		route, php := routes[p.Name]
		b.WriteString("location ~* \\.php[0-9]*/ { return 404; }\nlocation ~ (^|/)\\. { return 404; }\nlocation ~* (^|/)(vendor|node_modules|storage|config|database|tests|uploads)(/|$) { return 404; }\n")
		for _, upload := range route.Uploads {
			// Upload names have a deliberately restricted syntax, validated by FPM.
			for _, r := range upload {
				if !(r == '/' || r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
					return "", domain.E(domain.Invalid, "invalid upload path")
				}
			}
			if upload == "" || strings.Contains(upload, "..") {
				return "", domain.E(domain.Invalid, "invalid upload path")
			}
			fmt.Fprintf(&b, "location ~* ^/%s(/|$) { return 404; }\n", upload)
		}
		if php && p.ProxyPort != 0 {
			return "", domain.E(domain.Conflict, "project cannot enable FPM and proxy together")
		}
		if php {
			if route.Profile != "php" && route.Profile != "front-controller" {
				return "", domain.E(domain.Invalid, "invalid PHP profile")
			}
			socket, e := quote("unix:" + route.Socket)
			if e != nil {
				return "", e
			}
			if route.Profile == "front-controller" {
				b.WriteString("location = /index.php {\n")
			} else {
				b.WriteString("location ~ \\.php$ {\n")
			}
			// try_files obeys disable_symlinks; no PATH_INFO, arbitrary snippets or includes.
			fmt.Fprintf(&b, "try_files $uri =404;\nfastcgi_pass %s;\nfastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;\nfastcgi_param SCRIPT_NAME $fastcgi_script_name;\nfastcgi_param QUERY_STRING $query_string;\nfastcgi_param REQUEST_METHOD $request_method;\nfastcgi_param CONTENT_TYPE $content_type;\nfastcgi_param CONTENT_LENGTH $content_length;\nfastcgi_param REQUEST_URI $request_uri;\nfastcgi_param DOCUMENT_URI $document_uri;\nfastcgi_param DOCUMENT_ROOT $document_root;\nfastcgi_param SERVER_PROTOCOL $server_protocol;\nfastcgi_param SERVER_NAME $server_name;\nfastcgi_param SERVER_PORT $server_port;\nfastcgi_param REMOTE_ADDR $remote_addr;\nfastcgi_param GATEWAY_INTERFACE CGI/1.1;\nfastcgi_param REDIRECT_STATUS 200;\nfastcgi_param HTTP_PROXY '';\nfastcgi_param HTTPS $https if_not_empty;\nfastcgi_read_timeout 30s;\n}\n", socket)
		}
		b.WriteString("location ~* (\\.(php[0-9]*|phtml|phar|py|pyc|go|rb|pl|sh|sql|sqlite[0-9]*|log|pem|key|crt|bak|old|orig|swp|ini|toml|ya?ml|lock)|~)$ { return 404; }\nlocation ~* (^|/)(composer\\.json|package\\.json|artisan|Dockerfile|Makefile)$ { return 404; }\n")
		if p.ProxyPort != 0 {
			if p.ProxyPort == port {
				return "", domain.E(domain.Invalid, "proxy target cannot be the HTTP listener")
			}
			fmt.Fprintf(&b, "location / { proxy_pass http://127.0.0.1:%d; proxy_http_version 1.1; proxy_set_header Host $http_host; proxy_set_header X-Forwarded-Host $http_host; proxy_set_header X-Forwarded-For $remote_addr; proxy_set_header X-Forwarded-Proto $scheme; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $velnix_connection; proxy_connect_timeout 1s; proxy_read_timeout 60s; proxy_send_timeout 30s; proxy_buffering off; }\n", p.ProxyPort)
		} else if php {
			if route.Profile == "front-controller" {
				b.WriteString("location / { try_files $uri /index.php$is_args$args; }\n")
			} else {
				b.WriteString("location / { index index.php index.html; try_files $uri $uri/ =404; }\n")
			}
		} else {
			b.WriteString("location / { limit_except GET HEAD { deny all; } try_files $uri $uri/ =404; }\n")
		}
		b.WriteString("}\n")
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

// WithTLS retains independent HTTP listeners and adds optional loopback TLS.
func WithTLS(config string, httpPort, tlsPort int, cert, key string) (string, error) {
	if tlsPort < 1024 || tlsPort > 65535 || tlsPort == httpPort {
		return "", domain.E(domain.Invalid, "invalid HTTPS port")
	}
	c, e := quote(cert)
	if e != nil {
		return "", e
	}
	k, e := quote(key)
	if e != nil {
		return "", e
	}
	config = strings.ReplaceAll(config, fmt.Sprintf("listen 127.0.0.1:%d default_server;", httpPort), fmt.Sprintf("listen 127.0.0.1:%d default_server; listen 127.0.0.1:%d ssl default_server;", httpPort, tlsPort))
	config = strings.ReplaceAll(config, fmt.Sprintf("listen 127.0.0.1:%d;", httpPort), fmt.Sprintf("listen 127.0.0.1:%d; listen 127.0.0.1:%d ssl;", httpPort, tlsPort))
	config = strings.Replace(config, "http {\n", fmt.Sprintf("http {\nssl_certificate %s;\nssl_certificate_key %s;\nssl_protocols TLSv1.2 TLSv1.3;\nssl_session_tickets off;\n", c, k), 1)
	return config, nil
}
