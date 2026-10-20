//go:build linux

package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"velnix/internal/state"
)

func configureProxy(t *testing.T, s HTTPService, port int) {
	t.Helper()
	req := state.ProjectChange{Action: "proxy", Name: "one", ProxyPort: port}
	p, e := s.Store.PlanProject(req)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.ApplyProject(context.Background(), req, p.ID); e != nil {
		t.Fatal(e)
	}
}
func TestRealLoopbackProxyAndWebSocket(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	if _, e := exec.LookPath("nginx"); e != nil {
		t.Skip("installed Nginx required")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/socket" {
			if r.Header.Get("Upgrade") != "websocket" || !strings.EqualFold(r.Header.Get("Connection"), "upgrade") {
				http.Error(w, "missing upgrade", 400)
				return
			}
			conn, rw, e := w.(http.Hijacker).Hijack()
			if e != nil {
				return
			}
			defer conn.Close()
			fmt.Fprint(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			rw.Flush()
			buffer := make([]byte, 4)
			if _, e = io.ReadFull(rw, buffer); e == nil {
				rw.Write(buffer)
				rw.Flush()
			}
			return
		}
		fmt.Fprintf(w, "uri=%s method=%s host=%s proto=%s forwarded=%s", r.RequestURI, r.Method, r.Host, r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-For"))
	}))
	defer upstream.Close()
	port := upstream.Listener.Addr().(*net.TCPAddr).Port
	s := httpFixture(t)
	configureProxy(t, s, port)
	httpPort := freePort(t)
	startPool(t, s, HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: httpPort})
	code, body, e := requestHTTP(httpPort, "one.test", "/api/item?q=1")
	if e != nil || code != 200 || body != "uri=/api/item?q=1 method=GET host=one.test proto=http forwarded=127.0.0.1" {
		t.Fatalf("proxy %d %q %v", code, body, e)
	}
	// Forwarding headers are generated from this connection, not client claims.
	client := http.Client{Timeout: time.Second}
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/api", httpPort), strings.NewReader("value=1"))
	req.Host = "one.test"
	req.Header.Set("X-Forwarded-For", "evil")
	req.Header.Set("X-Forwarded-Proto", "https")
	response, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	client.CloseIdleConnections()
	if response.StatusCode != 200 || !strings.Contains(string(b), "method=POST") || !strings.Contains(string(b), "proto=http forwarded=127.0.0.1") {
		t.Fatalf("POST proxy: %d %s", response.StatusCode, b)
	}
	// Unknown domains and sensitive paths still fail before reaching the app.
	for _, tt := range []struct{ host, path string }{{"unknown.test", "/api"}, {"one.test", "/.env"}, {"one.test", "/config/secrets"}} {
		code, _, e := requestHTTP(httpPort, tt.host, tt.path)
		if e != nil || code != 404 {
			t.Fatalf("proxy deny: %d %v", code, e)
		}
	}
	conn, e := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", httpPort), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprint(conn, "GET /socket HTTP/1.1\r\nHost: one.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	request, _ := http.NewRequest("GET", "http://one.test/socket", nil)
	upgrade, e := http.ReadResponse(reader, request)
	if e != nil || upgrade.StatusCode != 101 {
		t.Fatalf("upgrade %v %v", upgrade, e)
	}
	conn.Write([]byte("ping"))
	echo := make([]byte, 4)
	if _, e = io.ReadFull(reader, echo); e != nil || string(echo) != "ping" {
		t.Fatalf("upgrade tunnel: %q %v", echo, e)
	}
	conn.Close()
	upstream.Close()
	code, _, e = requestHTTP(httpPort, "one.test", "/api")
	if e != nil || code != 502 {
		t.Fatalf("dead upstream: %d %v", code, e)
	}
	// Clearing proxy metadata takes effect only after an approved reload.
	configureProxy(t, s, 0)
	reload := HTTPRequest{Action: "reload"}
	p, e := s.Plan(context.Background(), reload)
	if e != nil {
		t.Fatal(e)
	}
	reload.Approval = p.ID
	if _, e = s.Apply(context.Background(), reload); e != nil {
		t.Fatal(e)
	}
	code, body, e = requestHTTP(httpPort, "one.test", "/")
	if e != nil || code != 200 || body != "one" {
		t.Fatalf("static restoration: %d %q %v", code, body, e)
	}
}
func TestProxyUnavailableAndListenerLoopRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	if _, e := exec.LookPath("nginx"); e != nil {
		t.Skip("installed Nginx required")
	}
	s := httpFixture(t)
	port := freePort(t)
	configureProxy(t, s, port)
	req := HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: port}
	if _, e := s.Plan(context.Background(), req); e == nil {
		t.Fatal("self proxy accepted")
	}
	req.Port = freePort(t)
	p, e := s.Plan(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	if _, e = s.Apply(context.Background(), req); e == nil || !strings.Contains(e.Error(), "app server") {
		t.Fatalf("unavailable upstream: %v", e)
	}
	if b, _ := s.Store.ReadHTTP(); b != nil {
		t.Fatal("failed preflight created listener evidence")
	}
	for _, port := range []int{-1, 1, 443, 65536} {
		if _, e = s.Store.PlanProject(state.ProjectChange{Action: "proxy", Name: "one", ProxyPort: port}); e == nil {
			t.Fatalf("accepted port %d", port)
		}
	}
}

func TestRealHTTPSProxy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	if _, e := exec.LookPath("nginx"); e != nil {
		t.Skip("Nginx required")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, r.Header.Get("X-Forwarded-Proto")) }))
	defer upstream.Close()
	s := httpFixture(t)
	configureProxy(t, s, upstream.Listener.Addr().(*net.TCPAddr).Port)
	service := TLSService{Store: s.Store}
	issueFixture(t, service, tlsFixture(t))
	_, _, ca, e := service.material(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	secure, plain := freePort(t), freePort(t)
	startPool(t, s, HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: plain, HTTPS: true, HTTPSPort: secure})
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "one.test", MinVersion: tls.VersionTLS12}}
	client := http.Client{Timeout: time.Second, Transport: transport}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://127.0.0.1:%d/api", secure), nil)
	req.Host = "one.test"
	response, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(b) != "https" {
		t.Fatalf("HTTPS proxy: %d %s", response.StatusCode, b)
	}
}
