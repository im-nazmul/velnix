//go:build linux

// Package mailpit implements a restricted local client, never a generic API proxy.
package mailpit

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/nativeexec"
)

const MaxRaw = 12 << 20 // 10 MiB SMTP payload plus provider-added headers.
const MaxMessages = 1000

type Credentials struct {
	User     string `json:"user"`
	Password string `json:"password"`
}
type Policy struct {
	SMTPPort    int `json:"smtp_port"`
	APIPort     int `json:"api_port"`
	MaxMessages int `json:"max_messages"`
	MaxDays     int `json:"max_days"`
}

func (p Policy) Validate() error {
	if p.SMTPPort < 1024 || p.SMTPPort > 65535 || p.APIPort < 1024 || p.APIPort > 65535 || p.SMTPPort == p.APIPort {
		return domain.E(domain.Invalid, "distinct SMTP/API ports must be 1024..65535; listeners always use 127.0.0.1")
	}
	if p.MaxMessages < 1 || p.MaxMessages > MaxMessages || p.MaxDays < 1 || p.MaxDays > 30 {
		return domain.E(domain.Invalid, "retention must be 1..1000 messages and 1..30 days")
	}
	return nil
}
func Select(path string) (domain.ExecutableSelection, error) {
	s, e := nativeexec.Snapshot(path, "1.33.0")
	if e != nil {
		return s, e
	}
	f, e := nativeexec.Open(s)
	if e != nil {
		return s, e
	}
	defer f.Close()
	info, e := buildinfo.Read(f)
	if e != nil || info.Main.Path != "github.com/axllent/mailpit" || (info.Main.Version != "v1.33.0" && info.Main.Version != "v1.33.0+dirty") {
		return s, domain.E(domain.Unavailable, "mail capture requires reviewed Mailpit v1.33.0 Go build metadata")
	}
	return s, nil
}

// Fixed shell text sets only the child's umask. No user text is shell code.
// Both provider and data directory are pinned descriptors, not mutable paths.
func Command(f, data *os.File, s domain.ExecutableSelection, p Policy, credentials Credentials, runtime string) *exec.Cmd {
	args := []string{"-c", `umask 077; exec /proc/self/fd/3 "$@"`, "velnix-mail",
		"--smtp", fmt.Sprintf("127.0.0.1:%d", p.SMTPPort), "--listen", fmt.Sprintf("127.0.0.1:%d", p.APIPort),
		"--allowed-hosts", fmt.Sprintf("127.0.0.1:%d", p.APIPort), "--database", "/proc/self/fd/4/mail.db",
		"--disable-version-check", "--smtp-disable-rdns", "--max-message-size", "10", "--max", strconv.Itoa(p.MaxMessages), "--max-age", strconv.Itoa(p.MaxDays) + "d", "--quiet"}
	c := exec.Command("/bin/sh", args...)
	c.ExtraFiles = []*os.File{f, data}
	c.Dir = runtime
	c.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + runtime, "TMPDIR=" + runtime, "MP_UI_AUTH=" + credentials.User + ":" + credentials.Password}
	c.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Setpgid: true}
	// Never expose provider diagnostics, which may include message content or URLs.
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	return c
}
func ValidID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return id != "latest"
}

type Address struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}
type Summary struct {
	Read    bool      `json:"Read"`
	ID      string    `json:"ID"`
	From    Address   `json:"From"`
	To      []Address `json:"To"`
	Subject string    `json:"Subject"`
	Created time.Time `json:"Created"`
	Size    int64     `json:"Size"`
}
type Page struct {
	Total    int       `json:"total"`
	Start    int       `json:"start"`
	Messages []Summary `json:"messages"`
}
type Client struct {
	port        int
	credentials Credentials
	http        *http.Client
}

func NewClient(port int, c Credentials) (*Client, error) {
	if port < 1024 || port > 65535 || c.User == "" || len(c.Password) < 32 {
		return nil, domain.E(domain.Invalid, "invalid owned mail API connection")
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 32 << 10, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: 3 * time.Second}
	return &Client{port, c, &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("mail API redirect refused") }}}, nil
}
func (c *Client) Close() { c.http.CloseIdleConnections() }
func (c *Client) call(ctx context.Context, method, path string, body []byte, limit int) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", c.port, path), bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(c.credentials.User, c.credentials.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := c.http.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, domain.E(domain.Unavailable, "owned mail API request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, domain.E(domain.Unavailable, "owned mail API returned HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, int64(limit+1)))
	if e != nil {
		return nil, domain.E(domain.Unavailable, "mail API response incomplete")
	}
	if len(b) > limit {
		return nil, domain.E(domain.Invalid, "mail API response exceeds bounded limit")
	}
	return b, nil
}
func (c *Client) List(ctx context.Context, start int) (Page, error) {
	var p Page
	if start < 0 || start > MaxMessages {
		return p, domain.E(domain.Invalid, "mail offset exceeds bounded range")
	}
	b, e := c.call(ctx, "GET", fmt.Sprintf("/api/v1/messages?start=%d&limit=50", start), nil, 1<<20)
	if e != nil {
		return p, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, domain.E(domain.Invalid, "invalid mail list response")
	}
	if p.Total < 0 || len(p.Messages) > 50 || p.Start != start {
		return p, domain.E(domain.Conflict, "mailbox exceeds bounded listing capacity or changed pagination")
	}
	for _, m := range p.Messages {
		if !ValidID(m.ID) || m.Created.IsZero() || m.Size < 0 {
			return p, domain.E(domain.Invalid, "invalid mail metadata")
		}
	}
	return p, nil
}
func (c *Client) Raw(ctx context.Context, id string) ([]byte, error) {
	if !ValidID(id) {
		return nil, domain.E(domain.Invalid, "invalid exact mail ID")
	}
	return c.call(ctx, "GET", "/api/v1/message/"+id+"/raw", nil, MaxRaw)
}
func (c *Client) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) > MaxMessages {
		return domain.E(domain.Invalid, "too many prune candidates")
	}
	for _, id := range ids {
		if !ValidID(id) {
			return domain.E(domain.Invalid, "invalid exact mail ID")
		}
	}
	b, _ := json.Marshal(struct {
		IDs []string `json:"IDs"`
	}{ids})
	_, e := c.call(ctx, "DELETE", "/api/v1/messages", b, 32<<10)
	return e
}
func (c *Client) All(ctx context.Context) ([]Summary, error) {
	var out []Summary
	seen := map[string]bool{}
	total := -1
	for start := 0; start <= MaxMessages; start += 50 {
		p, e := c.List(ctx, start)
		if e != nil {
			return nil, e
		}
		if total < 0 {
			total = p.Total
		}
		if total != p.Total {
			return nil, domain.E(domain.Conflict, "mailbox changed during snapshot; retry")
		}
		for _, m := range p.Messages {
			if seen[m.ID] {
				return nil, domain.E(domain.Conflict, "mailbox pagination changed; retry")
			}
			seen[m.ID] = true
			out = append(out, m)
		}
		if len(out) == total {
			return out, nil
		}
		if len(p.Messages) == 0 || len(out) > MaxMessages {
			return nil, domain.E(domain.Conflict, "mailbox snapshot exceeds bounded capacity")
		}
	}
	return nil, domain.E(domain.Conflict, "mailbox snapshot exceeds bounded capacity")
}

// Safe removes ANSI/control text, including C1 and bidi format characters.
func Safe(s string) string {
	s = strings.ToValidUTF8(s, "�")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || r >= 127 && r <= 159 || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s)
}
