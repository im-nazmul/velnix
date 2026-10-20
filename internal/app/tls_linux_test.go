//go:build linux

package app

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"velnix/internal/state"
)

func tlsFixture(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "mkcert-fixture")
	cmd := exec.Command("go", "build", "-o", exe, "./testdata/mkcertfixture")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build strict native adapter fixture: %v %s", e, b)
	}
	return exe
}
func issueFixture(t *testing.T, s TLSService, exe string) *TLSRecord {
	t.Helper()
	req := TLSRequest{Action: "issue", Executable: exe}
	v, e := s.Run(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = v.(TLSPlan).ID
	result, e := s.Run(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	return result.(*TLSRecord)
}
func TestTLSAdapterAndRealHTTPS(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged user")
	}
	exe := tlsFixture(t)
	t.Run("issuance and HTTPS", func(t *testing.T) {
		s := httpFixture(t)
		tlsService := TLSService{Store: s.Store}
		req := TLSRequest{Action: "issue", Executable: exe}
		value, e := tlsService.Run(context.Background(), req)
		if e != nil {
			t.Fatal(e)
		}
		if b, _ := s.Store.ReadWorkload("tls.json"); b != nil {
			t.Fatal("preview wrote TLS state")
		}
		req.Approval = "wrong"
		if _, e = tlsService.Run(context.Background(), req); e == nil {
			t.Fatal("stale approval accepted")
		}
		req.Approval = value.(TLSPlan).ID
		result, e := tlsService.Run(context.Background(), req)
		if e != nil {
			t.Fatal(e)
		}
		r := result.(*TLSRecord)
		if r.Phase != "ready" || len(r.Names) != 2 || r.Fingerprint == "" {
			t.Fatalf("bad evidence: %+v", r)
		}
		for _, name := range tlsFiles {
			info, e := os.Stat(filepath.Join(s.Store.Paths.State, r.Generation, name))
			if e != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("permissions %s %v", name, e)
			}
		}
		public, e := tlsService.PublicCA(context.Background())
		if e != nil || !strings.Contains(public, "BEGIN CERTIFICATE") || strings.Contains(public, "PRIVATE KEY") {
			t.Fatalf("public export: %v", e)
		}
		if _, e = exec.LookPath("nginx"); e != nil {
			t.Skip("Nginx required for real HTTPS")
		}
		port, secure := freePort(t), freePort(t)
		startPool(t, s, HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: port, HTTPS: true, HTTPSPort: secure})
		for _, name := range []string{"one", "two"} {
			verification, e := tlsService.Verify(context.Background(), name, secure)
			if e != nil || !verification.ExplicitCA || verification.LinuxSystemTrust || verification.LinuxError == "" || !strings.Contains(verification.WindowsBrowserTrust, "unverified") {
				t.Fatalf("verification %+v %v", verification, e)
			}
		}
		if code, body, e := requestHTTP(port, "one.test", "/"); e != nil || code != 200 || body != "one" {
			t.Fatalf("independent HTTP %d %q %v", code, body, e)
		}
		// External key edits must invalidate subsequent plans rather than overwrite.
		os.WriteFile(filepath.Join(s.Store.Paths.State, r.Generation, "key.pem"), []byte("external edit"), 0600)
		if _, e = s.Plan(context.Background(), HTTPRequest{Action: "reload"}); e == nil {
			t.Fatal("accepted modified private key")
		}
	})
	for _, name := range []string{"fail", "wrong"} {
		t.Run(name, func(t *testing.T) {
			s := httpFixture(t)
			service := TLSService{Store: s.Store}
			b, _ := os.ReadFile(exe)
			bad := filepath.Join(t.TempDir(), name)
			os.WriteFile(bad, b, 0700)
			req := TLSRequest{Action: "issue", Executable: bad}
			value, e := service.Run(context.Background(), req)
			if e != nil {
				t.Fatal(e)
			}
			req.Approval = value.(TLSPlan).ID
			if _, e = service.Run(context.Background(), req); e == nil {
				t.Fatal("invalid issuance accepted")
			}
			st, e := service.Status()
			if e != nil || st.Phase != "issuing" {
				t.Fatalf("missing interrupted evidence %+v %v", st, e)
			}
			if _, e = service.Run(context.Background(), TLSRequest{Action: "issue", Executable: exe}); e == nil {
				t.Fatal("replayed interrupted issuance")
			}
			recover := TLSRequest{Action: "recover"}
			plan, e := service.Run(context.Background(), recover)
			if e != nil {
				t.Fatal(e)
			}
			recover.Approval = plan.(TLSPlan).ID
			if _, e = service.Run(context.Background(), recover); e != nil {
				t.Fatal(e)
			}
			if _, e = os.Stat(filepath.Join(s.Store.Paths.State, st.Generation)); e != nil {
				t.Fatal("recovery removed interrupted artifacts")
			}
			next := issueFixture(t, service, exe)
			if next.Generation == st.Generation {
				t.Fatal("reused failed generation")
			}
		})
	}
}
func TestTLSMissingAndUnsafeEvidence(t *testing.T) {
	s := httpFixture(t)
	service := TLSService{Store: s.Store}
	if _, e := service.PublicCA(context.Background()); e == nil {
		t.Fatal("export without CA succeeded")
	}
	if _, e := s.Plan(context.Background(), HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: 8080, HTTPS: true, HTTPSPort: 8443}); e == nil {
		t.Fatal("HTTPS without certificate succeeded")
	}
	e := s.Store.WithHTTP(context.Background(), func(f state.HTTPFiles) error {
		return f.Write("tls.json", []byte(`{"schema":1,"phase":"ready","generation":"../../escape"}`))
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Status(); e == nil {
		t.Fatal("unsafe generation accepted")
	}
}

func TestTLSInstalledMkcert(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	exe := os.Getenv("VELNIX_TEST_MKCERT")
	var e error
	if exe == "" {
		exe, e = exec.LookPath("mkcert")
		if e != nil {
			t.Skip("real mkcert unavailable; set VELNIX_TEST_MKCERT to require an explicit binary")
		}
	} else if !filepath.IsAbs(exe) {
		t.Fatal("VELNIX_TEST_MKCERT must be absolute")
	}
	if info, err := os.Stat(exe); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("configured real mkcert is unavailable: %v", err)
	}
	exe, e = filepath.Abs(exe)
	if e != nil {
		t.Fatal(e)
	}
	s := httpFixture(t)
	service := TLSService{Store: s.Store}
	r := issueFixture(t, service, exe)
	t.Logf("real mkcert generation %s SANs %v expiry %s", r.Generation, r.Names, r.Expires)
	if _, e = exec.LookPath("nginx"); e != nil {
		t.Skip("real Nginx unavailable")
	}
	port := freePort(t)
	startPool(t, s, HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: freePort(t), HTTPS: true, HTTPSPort: port})
	for _, name := range []string{"one", "two"} {
		result, e := service.Verify(context.Background(), name, port)
		if e != nil || !result.ExplicitCA || result.LinuxSystemTrust {
			t.Fatalf("real mkcert verification %+v %v", result, e)
		}
	}
	public, e := service.PublicCA(context.Background())
	if e != nil || strings.Contains(public, "PRIVATE KEY") {
		t.Fatalf("public CA export: %v", e)
	}
	certificate, e := os.ReadFile(filepath.Join(s.Store.Paths.State, r.Generation, "cert.pem"))
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(certificate)
	if block == nil {
		t.Fatal("real mkcert leaf is not PEM")
	}
	leaf, e := x509.ParseCertificate(block.Bytes)
	if e != nil || !reflect.DeepEqual(leaf.DNSNames, []string{"one.test", "two.test"}) || !leaf.NotAfter.After(time.Now()) {
		t.Fatalf("real leaf SAN/expiry: %+v %v", leaf, e)
	}
	for _, name := range tlsFiles {
		info, e := os.Stat(filepath.Join(s.Store.Paths.State, r.Generation, name))
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("real artifact permissions %s: %v", name, e)
		}
	}
	// Real CA rotation keeps the old active listener intact until reviewed reload.
	next := issueFixture(t, service, exe)
	if next.Generation == r.Generation || next.Fingerprint == r.Fingerprint {
		t.Fatal("rotation reused old CA")
	}
	if _, e = service.Verify(context.Background(), "one", port); e == nil {
		t.Fatal("old endpoint was incorrectly accepted as the new CA/leaf")
	}
	reload := HTTPRequest{Action: "reload"}
	plan, e := s.Plan(context.Background(), reload)
	if e != nil {
		t.Fatal(e)
	}
	reload.Approval = plan.ID
	if _, e = s.Apply(context.Background(), reload); e != nil {
		t.Fatal(e)
	}
	result, e := service.Verify(context.Background(), "one", port)
	if e != nil || !result.ExplicitCA || result.LinuxSystemTrust {
		t.Fatalf("real CA rotation %+v %v", result, e)
	}
	if _, e = os.Stat(filepath.Join(s.Store.Paths.State, r.Generation, "rootCA.pem")); e != nil {
		t.Fatal("rotation deleted prior CA evidence")
	}
}
