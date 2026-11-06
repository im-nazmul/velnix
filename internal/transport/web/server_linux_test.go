//go:build linux

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"velnix/internal/app"
	"velnix/internal/domain"
	"velnix/internal/providers/mailpit"
)

type fakeBackend struct {
	reads, applied atomic.Int32
	stale          bool
	applyContext   error
}

func (f *fakeBackend) Read(context.Context, string) (app.DashboardData, error) {
	f.reads.Add(1)
	return app.DashboardData{Issues: []app.DashboardIssue{}}, nil
}
func (f *fakeBackend) Preview(_ context.Context, c app.DashboardChange) (app.DashboardReview, error) {
	if c.Operation != "initialize" {
		return app.DashboardReview{}, domain.E(domain.Invalid, "unsupported")
	}
	return app.DashboardReview{ID: "exact-plan", Change: c, Detail: map[string]any{"effects": []string{"initialize private state"}}}, nil
}
func (f *fakeBackend) Apply(ctx context.Context, _ app.DashboardChange, id string) (any, error) {
	f.applyContext = ctx.Err()
	if id != "exact-plan" || f.stale {
		return nil, domain.E(domain.Conflict, "plan changed")
	}
	f.applied.Add(1)
	return nil, nil
}
func (f *fakeBackend) MailPage(context.Context, int) (mailpit.Page, error) {
	return mailpit.Page{}, nil
}
func (f *fakeBackend) MailMessage(context.Context, string) (mailpit.Message, error) {
	return mailpit.Message{}, nil
}
func (f *fakeBackend) Attachment(context.Context, string, string) (mailpit.Attachment, error) {
	return mailpit.Attachment{Name: "../../evil.html", Bytes: []byte("<script>attack()</script>")}, nil
}
func fixture(t *testing.T) (*Server, *fakeBackend) {
	t.Helper()
	f := &fakeBackend{}
	s := New(context.Background(), "127.0.0.1:8090", strings.Repeat("a", 64), func() error { return nil }, f)
	t.Cleanup(s.Close)
	return s, f
}
func call(s *Server, method, path string, body any, cookies []*http.Cookie, csrf string, headers map[string]string) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Host = s.host
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.origin)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	for k, v := range headers {
		if k == "Host" {
			r.Host = v
		} else if v == "DELETE_HEADER" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func login(t *testing.T, s *Server) ([]*http.Cookie, string) {
	t.Helper()
	challenge := call(s, "GET", "/api/challenge", nil, nil, "", nil)
	if challenge.Code != 200 {
		t.Fatal(challenge.Code, challenge.Body.String())
	}
	var nonce map[string]string
	json.Unmarshal(challenge.Body.Bytes(), &nonce)
	response := call(s, "POST", "/api/login", map[string]string{"token": strings.Repeat("a", 64)}, challenge.Result().Cookies(), nonce["csrf"], nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var v map[string]string
	json.Unmarshal(response.Body.Bytes(), &v)
	var cookies []*http.Cookie
	for _, c := range response.Result().Cookies() {
		if c.Name == cookieName {
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Domain != "" || c.Path != "/" {
				t.Fatal("unsafe session cookie")
			}
			cookies = append(cookies, c)
		}
	}
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	if strings.Contains(response.Body.String(), strings.Repeat("a", 64)) {
		t.Fatal("bootstrap echoed")
	}
	return cookies, v["csrf"]
}
func TestAuthenticationHostOriginAndCSRF(t *testing.T) {
	s, f := fixture(t)
	for _, p := range []string{"/api/session", "/api/read/overview", "/api/mail/abc/attachment/1"} {
		if r := call(s, "GET", p, nil, nil, "", nil); r.Code != 401 {
			t.Fatal(p, r.Code)
		}
	}
	if f.reads.Load() != 0 {
		t.Fatal("unauthenticated backend read")
	}
	for _, h := range []map[string]string{{"Host": "evil.example"}, {"Origin": "https://evil.example"}, {"Origin": "null"}, {"Forwarded": "for=127.0.0.1"}, {"X-Forwarded-Host": ""}, {"Sec-Fetch-Site": "same-site"}, {"Sec-Fetch-Site": "cross-site"}} {
		if r := call(s, "GET", "/api/challenge", nil, nil, "", h); r.Code != 403 {
			t.Fatal(h, r.Code)
		}
	}
	if r := call(s, "GET", "/", nil, nil, "", nil); r.Code != 200 || !strings.Contains(r.Body.String(), "/assets/") || strings.Contains(r.Body.String(), "secret") {
		t.Fatal("embedded shell", r.Code)
	}
	cookies, csrf := login(t, s)
	if r := call(s, "GET", "/api/read/overview", nil, cookies, "", nil); r.Code != 200 || f.reads.Load() != 1 {
		t.Fatal(r.Code)
	}
	for _, h := range []map[string]string{{"Origin": "DELETE_HEADER"}, {"Origin": "http://localhost:8090"}, {"Content-Type": "text/plain"}, {"Origin": "https://evil.example"}} {
		r := call(s, "POST", "/api/plan", map[string]string{"operation": "initialize"}, cookies, csrf, h)
		if r.Code != 403 && r.Code != 415 {
			t.Fatal(h, r.Code)
		}
	}
	if r := call(s, "POST", "/api/plan", map[string]string{"operation": "initialize"}, cookies, "bad", nil); r.Code != 403 {
		t.Fatal("CSRF accepted", r.Code)
	}
	if r := call(s, "POST", "/api/plan", map[string]string{"operation": "initialize", "shell": "touch anything"}, cookies, csrf, nil); r.Code != 400 {
		t.Fatal("unknown fields", r.Code)
	}
	if r := call(s, "POST", "/api/plan", map[string]string{"operation": strings.Repeat("x", 33<<10)}, cookies, csrf, nil); r.Code != 413 {
		t.Fatal("oversized body", r.Code)
	}
	for _, p := range []string{"/assets/../../go.mod", "/assets/", "/api/read/overview?token=secret", "/api/mail?offset=0&unknown=x"} {
		r := call(s, "GET", p, nil, cookies, "", nil)
		if r.Code == 200 {
			t.Fatal("unexpected path allowed", p)
		}
	}
	r := call(s, "GET", "/api/mail/abc/attachment/1", nil, cookies, "", nil)
	if r.Code != 200 || r.Header().Get("Content-Type") != "application/octet-stream" || r.Header().Get("Content-Disposition") != "attachment; filename=evil.html" || r.Header().Get("Cache-Control") != "no-store" || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("unsafe download", r.Code, r.Header())
	}
	if !strings.Contains(r.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("missing CSP")
	}
	if r := call(s, "POST", "/api/logout", struct{}{}, cookies, csrf, nil); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := call(s, "GET", "/api/session", nil, cookies, "", nil); r.Code != 401 {
		t.Fatal("logout did not revoke")
	}
	if r := call(s, "GET", "/api/challenge", nil, nil, "", nil); r.Code != 401 {
		t.Fatal("bootstrap reusable")
	}
}
func preview(t *testing.T, s *Server, cookies []*http.Cookie, csrf string) map[string]any {
	t.Helper()
	r := call(s, "POST", "/api/plan", map[string]string{"operation": "initialize"}, cookies, csrf, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var p map[string]any
	json.Unmarshal(r.Body.Bytes(), &p)
	return p
}
func TestExactPlansSessionIsolationReplayAndExpiry(t *testing.T) {
	s, f := fixture(t)
	cookies, csrf := login(t, s)
	p := preview(t, s, cookies, csrf)
	apply := map[string]string{"ticket": p["ticket"].(string), "id": "exact-plan"}
	otherToken := randomToken()
	s.sessions[key(otherToken)] = &browserSession{CSRF: "other", Created: s.now(), Seen: s.now(), Plans: map[string]pendingPlan{}}
	other := []*http.Cookie{{Name: cookieName, Value: otherToken}}
	if r := call(s, "POST", "/api/apply", apply, other, "other", nil); r.Code != 409 {
		t.Fatal("cross-session plan accepted", r.Code)
	}
	if r := call(s, "POST", "/api/apply", apply, cookies, csrf, nil); r.Code != 200 || f.applied.Load() != 1 {
		t.Fatal(r.Code)
	}
	if r := call(s, "POST", "/api/apply", apply, cookies, csrf, nil); r.Code != 409 || f.applied.Load() != 1 {
		t.Fatal("plan replay")
	}
	p = preview(t, s, cookies, csrf)
	f.stale = true
	apply["ticket"] = p["ticket"].(string)
	if r := call(s, "POST", "/api/apply", apply, cookies, csrf, nil); r.Code != 409 {
		t.Fatal("stale backend plan applied")
	}
	f.stale = false
	p = preview(t, s, cookies, csrf)
	apply["ticket"] = p["ticket"].(string)
	now := s.now()
	s.now = func() time.Time { return now.Add(3 * time.Minute) }
	if r := call(s, "POST", "/api/apply", apply, cookies, csrf, nil); r.Code != 409 {
		t.Fatal("expired ticket")
	}
	s.now = func() time.Time { return now.Add(20 * time.Minute) }
	if r := call(s, "GET", "/api/session", nil, cookies, "", nil); r.Code != 401 {
		t.Fatal("idle expiry")
	}
}
func TestLoginRateBoundAndExpiredBootstrap(t *testing.T) {
	s, _ := fixture(t)
	challenge := call(s, "GET", "/api/challenge", nil, nil, "", nil)
	var nonce map[string]string
	json.Unmarshal(challenge.Body.Bytes(), &nonce)
	for i := 0; i < 6; i++ {
		r := call(s, "POST", "/api/login", map[string]string{"token": strings.Repeat("b", 64)}, challenge.Result().Cookies(), nonce["csrf"], nil)
		want := 401
		if i == 5 {
			want = 429
		}
		if r.Code != want {
			t.Fatal(i, r.Code)
		}
	}
	s.bootstrapUntil = s.now().Add(-time.Second)
	if r := call(s, "GET", "/api/challenge", nil, nil, "", nil); r.Code != 401 {
		t.Fatal("bootstrap expiry")
	}
}
func TestAcceptedApplySurvivesRequestDisconnect(t *testing.T) {
	s, f := fixture(t)
	cookies, csrf := login(t, s)
	p := preview(t, s, cookies, csrf)
	body := fmt.Sprintf(`{"ticket":%q,"id":"exact-plan"}`, p["ticket"])
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/api/apply", strings.NewReader(body)).WithContext(ctx)
	r.Host = s.host
	r.Header.Set("Origin", s.origin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || f.applyContext != nil || f.applied.Load() != 1 {
		t.Fatal("accepted action tied to disconnected request", w.Code, f.applyContext)
	}
}
