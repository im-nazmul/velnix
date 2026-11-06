//go:build linux

package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"velnix/internal/app"
	"velnix/internal/dashboardassets"
	"velnix/internal/domain"
	"velnix/internal/providers/mailpit"
	"velnix/internal/state"
)

type Backend interface {
	Read(context.Context, string) (app.DashboardData, error)
	Preview(context.Context, app.DashboardChange) (app.DashboardReview, error)
	Apply(context.Context, app.DashboardChange, string) (any, error)
	MailPage(context.Context, int) (mailpit.Page, error)
	MailMessage(context.Context, string) (mailpit.Message, error)
	Attachment(context.Context, string, string) (mailpit.Attachment, error)
}
type pendingPlan struct {
	Review  app.DashboardReview
	Expires time.Time
}
type browserSession struct {
	CSRF          string
	Created, Seen time.Time
	Plans         map[string]pendingPlan
	Busy          bool
}
type anonymous struct {
	CSRF    string
	Expires time.Time
}
type Server struct {
	mu             sync.Mutex
	origin, host   string
	backend        Backend
	assets         fs.FS
	bootstrap      [32]byte
	bootstrapUsed  bool
	bootstrapUntil time.Time
	consume        func() error
	timer          *time.Timer
	sessions       map[string]*browserSession
	anonymous      map[string]anonymous
	failures       int
	failureWindow  time.Time
	now            func() time.Time
	ctx            context.Context
	work           chan struct{}
}

const cookieName = "velnix_session"
const loginCookie = "velnix_login"

func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic("secure random source unavailable")
	}
	return hex.EncodeToString(b)
}
func key(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func New(ctx context.Context, address, secret string, consume func() error, backend Backend) *Server {
	now := time.Now()
	s := &Server{origin: "http://" + address, host: address, backend: backend, assets: dashboardassets.Files(), bootstrap: sha256.Sum256([]byte(secret)), bootstrapUntil: now.Add(5 * time.Minute), consume: consume, sessions: map[string]*browserSession{}, anonymous: map[string]anonymous{}, now: time.Now, ctx: ctx, work: make(chan struct{}, 8)}
	s.timer = time.AfterFunc(5*time.Minute, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.bootstrapUsed {
			s.bootstrapUsed = true
			_ = s.consume()
		}
	})
	return s
}
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timer.Stop()
	s.bootstrapUsed = true
	s.sessions = map[string]*browserSession{}
	s.anonymous = map[string]anonymous{}
	_ = s.consume()
}
func respond(w http.ResponseWriter, status int, value any) {
	b, e := json.Marshal(value)
	if e != nil {
		http.Error(w, "Response unavailable", 500)
		return
	}
	if len(b) > 2<<20 {
		http.Error(w, "Response exceeds dashboard limit", http.StatusRequestEntityTooLarge)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
func failure(w http.ResponseWriter, status int, code domain.Code, message string) {
	respond(w, status, map[string]any{"error": map[string]any{"code": code, "message": mailpit.Safe(message)}})
}
func backendError(w http.ResponseWriter, e error) {
	status := http.StatusInternalServerError
	code := domain.Failure
	message := "Operation failed; inspect state before retrying."
	var de *domain.Error
	if errors.As(e, &de) {
		code = de.Code
		message = de.Message
		switch de.Code {
		case domain.Invalid:
			status = 400
		case domain.Denied:
			status = 403
		case domain.Conflict:
			status = 409
		case domain.Unavailable:
			status = 503
		}
	}
	if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
		status = 503
		message = "Operation timed out or was cancelled. Inspect current state before retrying."
	}
	failure(w, status, code, message)
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	typ, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" || len(params) > 1 || (len(params) == 1 && strings.ToLower(params["charset"]) != "utf-8") {
		return domain.E(domain.Invalid, "application/json content type required")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return domain.E(domain.Invalid, "invalid or oversized JSON request")
	}
	if d.Decode(new(any)) != io.EOF {
		return domain.E(domain.Invalid, "request must contain exactly one JSON object")
	}
	return nil
}
func cookie(r *http.Request, name string) string {
	values := r.CookiesNamed(name)
	if len(values) != 1 || len(values[0].Value) != 64 {
		return ""
	}
	return values[0].Value
}
func setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'; object-src 'none'")
	if r.Host != s.host || r.URL.IsAbs() || len(r.Header.Values("Forwarded")) > 0 || len(r.Header.Values("X-Forwarded-Host")) > 0 || len(r.Header.Values("X-Forwarded-For")) > 0 {
		failure(w, 403, domain.Denied, "Unrecognized local authority")
		return
	}
	origin := r.Header.Values("Origin")
	if len(origin) > 1 || (len(origin) == 1 && origin[0] != s.origin) {
		failure(w, 403, domain.Denied, "Origin rejected")
		return
	}
	site := r.Header.Get("Sec-Fetch-Site")
	if site != "" && site != "same-origin" && site != "none" {
		failure(w, 403, domain.Denied, "Cross-site request rejected")
		return
	}
	if r.Method != "GET" && r.Method != "POST" {
		failure(w, 405, domain.Invalid, "Method unavailable")
		return
	}
	if r.Method == "POST" && (len(origin) != 1 || origin[0] != s.origin) {
		failure(w, 403, domain.Denied, "Exact Origin required")
		return
	}
	if r.Method == "POST" {
		typ, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || typ != "application/json" || len(params) > 1 || (len(params) == 1 && strings.ToLower(params["charset"]) != "utf-8") {
			failure(w, 415, domain.Invalid, "application/json required")
			return
		}
		if r.ContentLength > 32<<10 {
			failure(w, 413, domain.Invalid, "Request body too large")
			return
		}
	}
	if r.URL.RawQuery != "" && r.URL.Path != "/api/mail" {
		failure(w, 400, domain.Invalid, "Unexpected query parameters")
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		s.static(w, r)
		return
	}
	select {
	case s.work <- struct{}{}:
		defer func() { <-s.work }()
	default:
		failure(w, 429, domain.Unavailable, "Dashboard is busy; retry reads later")
		return
	}
	if r.URL.Path == "/api/challenge" && r.Method == "GET" {
		s.challenge(w, r)
		return
	}
	if r.URL.Path == "/api/login" && r.Method == "POST" {
		s.login(w, r)
		return
	}
	sessionKey := key(cookie(r, cookieName))
	s.mu.Lock()
	session, ok := s.sessions[sessionKey]
	now := s.now()
	if ok && (now.Sub(session.Seen) > 15*time.Minute || now.Sub(session.Created) > 2*time.Hour) {
		delete(s.sessions, sessionKey)
		ok = false
	}
	if !ok {
		s.mu.Unlock()
		failure(w, 401, domain.Denied, "Session expired or unavailable. Restart velnix ui to authenticate again.")
		return
	}
	if r.Method == "POST" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(session.CSRF)) != 1 {
		s.mu.Unlock()
		failure(w, 403, domain.Denied, "CSRF token rejected")
		return
	}
	session.Seen = now
	csrf := session.CSRF
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	switch {
	case r.URL.Path == "/api/session" && r.Method == "GET":
		respond(w, 200, map[string]any{"csrf": csrf, "idle_seconds": 900})
	case r.URL.Path == "/api/logout" && r.Method == "POST":
		var empty struct{}
		if e := decode(w, r, &empty); e != nil {
			backendError(w, e)
			return
		}
		s.mu.Lock()
		delete(s.sessions, sessionKey)
		s.mu.Unlock()
		setCookie(w, cookieName, "", -1)
		respond(w, 200, map[string]bool{"logged_out": true})
	case strings.HasPrefix(r.URL.Path, "/api/read/") && r.Method == "GET":
		v, e := s.backend.Read(ctx, strings.TrimPrefix(r.URL.Path, "/api/read/"))
		if e != nil {
			backendError(w, e)
			return
		}
		respond(w, 200, v)
	case r.URL.Path == "/api/plan" && r.Method == "POST":
		s.plan(w, r, sessionKey, session)
	case r.URL.Path == "/api/apply" && r.Method == "POST":
		s.apply(w, r, sessionKey, session)
	case r.URL.Path == "/api/mail" && r.Method == "GET":
		q, e := url.ParseQuery(r.URL.RawQuery)
		if e != nil || len(q) > 1 || len(q["offset"]) > 1 {
			failure(w, 400, domain.Invalid, "Invalid inbox query")
			return
		}
		for name := range q {
			if name != "offset" {
				failure(w, 400, domain.Invalid, "Invalid inbox query")
				return
			}
		}
		offset := 0
		if q.Get("offset") != "" {
			offset, e = strconv.Atoi(q.Get("offset"))
			if e != nil {
				failure(w, 400, domain.Invalid, "Invalid inbox offset")
				return
			}
		}
		v, e := s.backend.MailPage(ctx, offset)
		if e != nil {
			backendError(w, e)
			return
		}
		respond(w, 200, v)
	case strings.HasPrefix(r.URL.Path, "/api/mail/") && r.Method == "GET":
		s.mail(w, r, ctx)
	default:
		failure(w, 404, domain.Invalid, "Endpoint unavailable")
	}
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		failure(w, 405, domain.Invalid, "Static route is read-only")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) || (name != "index.html" && name != "favicon.svg" && !strings.HasPrefix(name, "assets/")) {
		http.NotFound(w, r)
		return
	}
	b, e := fs.ReadFile(s.assets, name)
	if e != nil || len(b) > 1<<20 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
	_, _ = w.Write(b)
}
func (s *Server) challenge(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, a := range s.anonymous {
		if now.After(a.Expires) {
			delete(s.anonymous, id)
		}
	}
	if s.bootstrapUsed || now.After(s.bootstrapUntil) {
		failure(w, 401, domain.Denied, "Bootstrap consumed or expired. Restart velnix ui for a fresh login.")
		return
	}
	if len(s.anonymous) >= 32 {
		failure(w, 429, domain.Unavailable, "Too many login challenges; wait for expiry")
		return
	}
	token, csrf := randomToken(), randomToken()
	s.anonymous[key(token)] = anonymous{csrf, now.Add(5 * time.Minute)}
	setCookie(w, loginCookie, token, 300)
	respond(w, 200, map[string]string{"csrf": csrf})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if e := decode(w, r, &body); e != nil {
		backendError(w, e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	anonID := key(cookie(r, loginCookie))
	anon, ok := s.anonymous[anonID]
	if !ok || now.After(anon.Expires) || subtle.ConstantTimeCompare([]byte(anon.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
		failure(w, 403, domain.Denied, "Login CSRF token rejected")
		return
	}
	if now.Sub(s.failureWindow) > time.Minute {
		s.failureWindow = now
		s.failures = 0
	}
	if s.failures >= 5 {
		failure(w, 429, domain.Denied, "Login attempts limited; wait one minute")
		return
	}
	candidate := sha256.Sum256([]byte(body.Token))
	if s.bootstrapUsed || now.After(s.bootstrapUntil) || len(body.Token) != 64 || subtle.ConstantTimeCompare(candidate[:], s.bootstrap[:]) != 1 {
		s.failures++
		failure(w, 401, domain.Denied, "Bootstrap secret invalid, expired or consumed")
		return
	}
	if e := s.consume(); e != nil {
		failure(w, 503, domain.Unavailable, "Private bootstrap file changed or cannot be removed; inspect runtime state")
		return
	}
	s.bootstrapUsed = true
	s.bootstrap = [32]byte{}
	s.timer.Stop()
	s.anonymous = map[string]anonymous{}
	token, csrf := randomToken(), randomToken()
	s.sessions[key(token)] = &browserSession{CSRF: csrf, Created: now, Seen: now, Plans: map[string]pendingPlan{}}
	setCookie(w, loginCookie, "", -1)
	setCookie(w, cookieName, token, 7200)
	respond(w, 200, map[string]string{"csrf": csrf})
}
func (s *Server) plan(w http.ResponseWriter, r *http.Request, id string, session *browserSession) {
	var change app.DashboardChange
	if e := decode(w, r, &change); e != nil {
		backendError(w, e)
		return
	}
	s.mu.Lock()
	if s.sessions[id] != session || session.Busy {
		s.mu.Unlock()
		failure(w, 409, domain.Conflict, "Session is unavailable or applying another plan")
		return
	}
	session.Busy = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); session.Busy = false; s.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	review, e := s.backend.Preview(ctx, change)
	if e != nil {
		backendError(w, e)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] != session {
		failure(w, 401, domain.Denied, "Session ended while preparing plan")
		return
	}
	now := s.now()
	for k, p := range session.Plans {
		if now.After(p.Expires) {
			delete(session.Plans, k)
		}
	}
	if len(session.Plans) >= 8 {
		failure(w, 429, domain.Unavailable, "Pending plan limit reached; let old previews expire")
		return
	}
	ticket := randomToken()
	expires := now.Add(2 * time.Minute)
	session.Plans[ticket] = pendingPlan{review, expires}
	respond(w, 200, map[string]any{"ticket": ticket, "expires": expires, "review": review})
}
func (s *Server) apply(w http.ResponseWriter, r *http.Request, id string, session *browserSession) {
	var body struct {
		Ticket string `json:"ticket"`
		ID     string `json:"id"`
	}
	if e := decode(w, r, &body); e != nil {
		backendError(w, e)
		return
	}
	s.mu.Lock()
	pending, ok := session.Plans[body.Ticket]
	if !ok || s.sessions[id] != session || session.Busy || s.now().After(pending.Expires) || body.ID != pending.Review.ID {
		s.mu.Unlock()
		failure(w, 409, domain.Conflict, "Plan unavailable, expired or already applied. Refresh and review a new plan.")
		return
	}
	delete(session.Plans, body.Ticket)
	session.Busy = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); session.Busy = false; s.mu.Unlock() }()
	// An accepted mutation is independent of an unreliable browser disconnect.
	// Never retry it automatically. Backend journals govern uncertain outcomes.
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	_, e := s.backend.Apply(ctx, pending.Review.Change, pending.Review.ID)
	if e != nil {
		backendError(w, e)
		return
	}
	respond(w, 200, map[string]bool{"applied": true})
}
func (s *Server) mail(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/mail/"), "/")
	if len(parts) < 1 || !mailpit.ValidID(parts[0]) {
		failure(w, 400, domain.Invalid, "Exact message ID required")
		return
	}
	if len(parts) == 1 {
		v, e := s.backend.MailMessage(ctx, parts[0])
		if e != nil {
			backendError(w, e)
			return
		}
		respond(w, 200, v)
		return
	}
	if len(parts) != 3 || parts[1] != "attachment" {
		failure(w, 404, domain.Invalid, "Mail endpoint unavailable")
		return
	}
	a, e := s.backend.Attachment(ctx, parts[0], parts[2])
	if e != nil {
		backendError(w, e)
		return
	}
	name := mailpit.Safe(path.Base(strings.ReplaceAll(a.Name, "\\", "/")))
	if name == "" || name == "." || name == ".." || name == "/" {
		name = "attachment.bin"
	}
	if len(name) > 180 {
		name = "attachment.bin"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(a.Bytes)))
	_, _ = w.Write(a.Bytes)
}

type cappedListener struct {
	net.Listener
	slots chan struct{}
}
type cappedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *cappedConn) Close() error { e := c.Conn.Close(); c.once.Do(c.release); return e }
func (l cappedListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		select {
		case l.slots <- struct{}{}:
			return &cappedConn{Conn: c, release: func() { <-l.slots }}, nil
		default:
			c.Close()
		}
	}
}

// Serve receives an already reviewed launch plan and owns only its local listener.
func Serve(ctx context.Context, p app.DashboardLaunch, backend Backend, ready func(string, string) error) (result error) {
	host, portText, err := net.SplitHostPort(p.Address)
	port, _ := strconv.Atoi(portText)
	if err != nil || host != "127.0.0.1" || port < 1024 || port > 65535 {
		return domain.E(domain.Invalid, "UI listener must use the reviewed numeric loopback address")
	}
	listener, e := net.Listen("tcp4", p.Address)
	if e != nil {
		return domain.E(domain.Conflict, "Dashboard port unavailable; no listener was replaced")
	}
	defer listener.Close()
	secret := randomToken()
	name := "ui-" + randomToken()[:32]
	bootstrap, e := state.CreateUIBootstrap(p.Runtime.Base, name, secret)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, bootstrap.Close()) }()
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	s := New(life, p.Address, secret, bootstrap.Consume, backend)
	defer s.Close()
	secret = ""
	server := &http.Server{Handler: s, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	done := make(chan error, 1)
	go func() { done <- server.Serve(cappedListener{listener, make(chan struct{}, 64)}) }()
	if e = ready("http://"+p.Address, bootstrap.Path); e != nil {
		server.Close()
		<-done
		return e
	}
	select {
	case <-ctx.Done():
		cancel()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if e = server.Shutdown(shutdown); e != nil {
			_ = server.Close()
		}
		<-done
		return ctx.Err()
	case e = <-done:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	}
}
