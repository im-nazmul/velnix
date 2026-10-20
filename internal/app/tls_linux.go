//go:build linux

package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/providers/nginx"
	"velnix/internal/state"
)

type TLSRecord struct {
	Schema      int               `json:"schema"`
	CAPEM       string            `json:"ca_pem,omitempty"`
	Phase       string            `json:"phase"`
	Generation  string            `json:"generation"`
	Names       []string          `json:"names"`
	Digests     map[string]string `json:"digests,omitempty"`
	Fingerprint string            `json:"ca_sha256,omitempty"`
	Expires     time.Time         `json:"expires"`
}
type TLSRequest struct {
	Action, Executable, Approval string
	DryRun                       bool
}
type TLSPlan struct {
	ID          string                     `json:"id"`
	Executable  domain.ExecutableSelection `json:"executable"`
	Before      string                     `json:"before"`
	Registry    string                     `json:"registry_digest"`
	Names       []string                   `json:"names"`
	Argv        []string                   `json:"argv"`
	Environment []string                   `json:"environment"`
	Directory   string                     `json:"directory"`
	Effects     []string                   `json:"effects"`
	Limitations string                     `json:"limitations"`
}
type TLSService struct{ Store state.Store }

var tlsFiles = []string{"rootCA.pem", "rootCA-key.pem", "cert.pem", "key.pem"}

func readTLS(b []byte) (*TLSRecord, error) {
	if b == nil {
		return nil, nil
	}
	var r TLSRecord
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(&r); e != nil {
		return nil, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return nil, domain.E(domain.Invalid, "trailing TLS record data")
	}
	if r.Schema != 1 || (r.Phase != "issuing" && r.Phase != "ready" && r.Phase != "abandoned") || len(r.Generation) != 20 || !strings.HasPrefix(r.Generation, "tls-") {
		return nil, domain.E(domain.Invalid, "invalid TLS evidence")
	}
	for _, c := range r.Generation[4:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return nil, domain.E(domain.Invalid, "invalid TLS generation")
		}
	}
	for _, n := range r.Names {
		if !strings.HasSuffix(n, ".test") || !domain.ValidProjectName(strings.TrimSuffix(n, ".test")) {
			return nil, domain.E(domain.Invalid, "invalid TLS names")
		}
	}
	return &r, nil
}
func (s TLSService) Status() (*TLSRecord, error) {
	b, e := s.Store.ReadWorkload("tls.json")
	if e != nil {
		return nil, e
	}
	return readTLS(b)
}
func (s TLSService) plan(req TLSRequest) (TLSPlan, error) {
	var p TLSPlan
	st, e := s.Store.Status()
	if e != nil {
		return p, e
	}
	if !st.Initialized || st.RecoveryRequired {
		return p, domain.E(domain.Conflict, "initialize/recover registry first")
	}
	b, e := s.Store.ReadWorkload("tls.json")
	if e != nil {
		return p, e
	}
	r, e := readTLS(b)
	if e != nil {
		return p, e
	}
	p.Before = hash(b)
	p.Registry = hash(data(st.Registry))
	p.Directory = s.Store.Paths.State
	p.Limitations = "Private untrusted CA per generation; no system/Windows/browser trust or hosts changes. Previous generations are preserved. Recovery abandons issuance, never replays it. Untrust each fingerprint manually before removing its files."
	if req.Action == "recover" {
		if r != nil && r.Phase == "issuing" {
			p.Effects = []string{"mark interrupted issuance abandoned; preserve all files"}
		}
		p.ID = hash(data(p))
		return p, nil
	}
	if req.Action != "issue" {
		return p, domain.E(domain.Invalid, "unknown TLS action")
	}
	if os.Geteuid() == 0 {
		return p, domain.E(domain.Denied, "TLS issuance must run unprivileged")
	}
	if r != nil && r.Phase == "issuing" {
		return p, domain.E(domain.Conflict, "interrupted TLS issuance; review tls recover")
	}
	for _, project := range st.Registry.Projects {
		p.Names = append(p.Names, project.Domain)
	}
	sort.Strings(p.Names)
	if len(p.Names) == 0 {
		return p, domain.E(domain.Invalid, "link at least one project before TLS issuance")
	}
	p.Executable, e = nativeexec.Snapshot(req.Executable, "1.4.4")
	if e != nil {
		return p, e
	}
	p.Argv = append([]string{"-cert-file", "cert.pem", "-key-file", "key.pem"}, p.Names...)
	p.Environment = []string{"PATH=/usr/bin:/bin", "LANG=C", "CAROOT=<new private generation>", "TRUST_STORES=none"}
	p.Effects = []string{"verify selected mkcert -version is v1.4.4", "journal issuance intent and create exclusive private generation", "run mkcert with explicit names and isolated CAROOT; never -install", "validate exact SANs, certificate chain, expiry, key pairs and private permissions", "publish ready evidence; preserve previous generation and CA"}
	p.ID = hash(data(p))
	return p, nil
}
func (s TLSService) Run(ctx context.Context, req TLSRequest) (any, error) {
	if req.DryRun && req.Approval != "" {
		return nil, domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
	}
	switch req.Action {
	case "status":
		return s.Status()
	case "export-ca":
		return s.PublicCA(ctx)
	}
	p, e := s.plan(req)
	if e != nil {
		return nil, e
	}
	if req.Approval == "" {
		return p, nil
	}
	if p.ID != req.Approval {
		return nil, domain.E(domain.Conflict, "TLS plan changed; preview again")
	}
	e = s.Store.WithHTTP(ctx, func(f state.HTTPFiles) error {
		current, e := s.plan(req)
		if e != nil {
			return e
		}
		if current.ID != req.Approval {
			return domain.E(domain.Conflict, "TLS plan changed; preview again")
		}
		if req.Action == "recover" {
			if len(p.Effects) == 0 {
				return nil
			}
			b, e := f.Read("tls.json")
			if e != nil {
				return e
			}
			r, e := readTLS(b)
			if e != nil {
				return e
			}
			r.Phase = "abandoned"
			return f.Write("tls.json", data(r))
		}
		binary, e := nativeexec.Open(p.Executable)
		if e != nil {
			return e
		}
		defer binary.Close()
		step, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		command := nativeexec.Command(binary, p.Executable, "-version")
		command.Dir = "/"
		command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
		var output nginx.Tail
		command.Stdout = &output
		command.Stderr = &output
		if e = runTLSCommand(step, command); e != nil {
			return e
		}
		if output.String() != "v1.4.4" {
			return domain.E(domain.Unavailable, "unsupported mkcert adapter version; requires v1.4.4")
		}
		r := TLSRecord{Schema: 1, Phase: "issuing", Generation: "tls-" + p.ID[:16], Names: p.Names}
		before, e := f.Read("tls.json")
		if e != nil {
			return e
		}
		if e = f.Write("tls-before.json", before); e != nil {
			return e
		}
		if e = f.Write("tls.json", data(r)); e != nil {
			return e
		}
		e = f.WithDirectory(r.Generation, true, func(g state.HTTPFiles) error {
			dir := filepath.Join(s.Store.Paths.State, r.Generation)
			command := nativeexec.Command(binary, p.Executable, p.Argv...)
			command.Dir = dir
			command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "CAROOT=" + dir, "TRUST_STORES=none"}
			// mkcert output can mention filenames and trust hints; it is drained but not
			// exposed as a trust result or mixed with structured CLI output.
			var tail nginx.Tail
			command.Stdout = &tail
			command.Stderr = &tail
			if e := runTLSCommand(step, command); e != nil {
				return domain.E(domain.Unavailable, "mkcert issuance failed: %v; inspect tls status/recover", e)
			}
			for _, name := range tlsFiles {
				if e := g.ProtectGenerated(name); e != nil {
					return e
				}
			}
			_, _, e := validateTLS(g, &r, true)
			return e
		})
		if e != nil {
			return e
		}
		r.Phase = "ready"
		return f.Write("tls.json", data(r))
	})
	if e != nil {
		return nil, e
	}
	return s.Status()
}
func validateTLS(g state.HTTPFiles, r *TLSRecord, issuing bool) (tls.Certificate, *x509.Certificate, error) {
	var zero tls.Certificate
	files := map[string][]byte{}
	digests := map[string]string{}
	for _, name := range tlsFiles {
		b, e := g.Read(name)
		if e != nil {
			return zero, nil, e
		}
		if len(b) == 0 {
			return zero, nil, domain.E(domain.Unavailable, "TLS artifact missing")
		}
		files[name] = b
		digests[name] = hash(b)
	}
	if !issuing && !reflect.DeepEqual(r.Digests, digests) {
		return zero, nil, domain.E(domain.Conflict, "TLS files changed; preserve evidence")
	}
	parse := func(b []byte) (*x509.Certificate, error) {
		block, rest := pem.Decode(b)
		if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
			return nil, fmt.Errorf("invalid certificate PEM")
		}
		return x509.ParseCertificate(block.Bytes)
	}
	ca, e := parse(files["rootCA.pem"])
	if e != nil {
		return zero, nil, e
	}
	leaf, e := parse(files["cert.pem"])
	if e != nil {
		return zero, nil, e
	}
	pair, e := tls.X509KeyPair(files["cert.pem"], files["key.pem"])
	if e != nil {
		return zero, nil, e
	}
	if _, e = tls.X509KeyPair(files["rootCA.pem"], files["rootCA-key.pem"]); e != nil {
		return zero, nil, e
	}
	if !ca.IsCA || ca.CheckSignatureFrom(ca) != nil || leaf.IsCA {
		return zero, nil, fmt.Errorf("invalid private CA chain")
	}
	names := append([]string(nil), leaf.DNSNames...)
	sort.Strings(names)
	if !reflect.DeepEqual(names, r.Names) || len(leaf.IPAddresses) != 0 || len(leaf.EmailAddresses) != 0 || len(leaf.URIs) != 0 {
		return zero, nil, fmt.Errorf("certificate SANs differ from approved exact names")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); e != nil {
		return zero, nil, e
	}
	if issuing {
		r.Digests = digests
		r.Fingerprint = hash(ca.Raw)
		r.Expires = leaf.NotAfter
		r.CAPEM = string(files["rootCA.pem"])
	} else if r.Fingerprint != hash(ca.Raw) || !r.Expires.Equal(leaf.NotAfter) || r.CAPEM != string(files["rootCA.pem"]) {
		return zero, nil, fmt.Errorf("TLS certificate evidence mismatch")
	}
	return pair, ca, nil
}
func (s TLSService) material(ctx context.Context) (*TLSRecord, tls.Certificate, *x509.Certificate, error) {
	var r *TLSRecord
	var pair tls.Certificate
	var ca *x509.Certificate
	e := s.Store.WithHTTP(ctx, func(f state.HTTPFiles) error {
		b, e := f.Read("tls.json")
		if e != nil {
			return e
		}
		r, e = readTLS(b)
		if e != nil {
			return e
		}
		if r == nil || r.Phase != "ready" {
			return domain.E(domain.Unavailable, "no ready TLS generation")
		}
		return f.WithDirectory(r.Generation, false, func(g state.HTTPFiles) error { pair, ca, e = validateTLS(g, r, false); return e })
	})
	return r, pair, ca, e
}
func (s TLSService) PublicCA(ctx context.Context) (string, error) {
	_, _, ca, e := s.material(ctx)
	if e != nil {
		return "", e
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})), nil
}

type TLSVerification struct {
	ExplicitCA          bool   `json:"explicit_ca"`
	LinuxSystemTrust    bool   `json:"linux_system_trust"`
	LinuxError          string `json:"linux_error,omitempty"`
	WindowsBrowserTrust string `json:"windows_browser_trust"`
	Fingerprint         string `json:"ca_sha256"`
}

func (s TLSService) Verify(ctx context.Context, name string, port int) (TLSVerification, error) {
	result := TLSVerification{WindowsBrowserTrust: "unverified; manual Windows/browser check required"}
	if !domain.ValidProjectName(name) || port < 1024 || port > 65535 {
		return result, domain.E(domain.Invalid, "verify requires a project name and unprivileged port")
	}
	r, _, ca, e := s.material(ctx)
	if e != nil {
		return result, e
	}
	host := name + ".test"
	if !sort.StringsAreSorted(r.Names) {
		return result, fmt.Errorf("invalid certificate names")
	}
	found := false
	for _, n := range r.Names {
		found = found || n == host
	}
	if !found {
		return result, domain.E(domain.Invalid, "name is not covered by current TLS generation")
	}
	result.Fingerprint = r.Fingerprint
	check := func(roots *x509.CertPool) error {
		client := http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		}}}
		defer client.CloseIdleConnections()
		request, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+"/", nil)
		response, e := client.Do(request)
		if e != nil {
			return e
		}
		defer response.Body.Close()
		if response.TLS == nil || len(response.TLS.PeerCertificates) == 0 {
			return fmt.Errorf("endpoint has no TLS certificate")
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: response.TLS.PeerCertificates[0].Raw})
		if hash(certPEM) != r.Digests["cert.pem"] {
			return fmt.Errorf("endpoint serves a different certificate")
		}
		return nil
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if e = check(roots); e != nil {
		return result, e
	}
	result.ExplicitCA = true
	if e = check(nil); e != nil {
		result.LinuxError = e.Error()
	} else {
		result.LinuxSystemTrust = true
	}
	return result, nil
}

func runTLSCommand(ctx context.Context, cmd *exec.Cmd) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case e := <-done:
		return e
	case <-ctx.Done():
		cmd.Process.Kill()
		<-done
		return ctx.Err()
	}
}
func (s TLSService) materialLocked(f state.HTTPFiles) (*TLSRecord, error) {
	b, e := f.Read("tls.json")
	if e != nil {
		return nil, e
	}
	r, e := readTLS(b)
	if e != nil {
		return nil, e
	}
	if r == nil || r.Phase != "ready" {
		return nil, domain.E(domain.Unavailable, "issue a ready TLS generation first")
	}
	e = f.WithDirectory(r.Generation, false, func(g state.HTTPFiles) error { _, _, e := validateTLS(g, r, false); return e })
	return r, e
}
