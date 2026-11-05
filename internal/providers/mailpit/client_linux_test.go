//go:build linux

package mailpit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"velnix/internal/domain"
)

func TestRestrictedClientAndEnvironment(t *testing.T) {
	credentials := Credentials{"velnix", strings.Repeat("b", 64)}
	var mode atomic.Value
	mode.Store("normal")
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		u, p, ok := r.BasicAuth()
		if !ok || u != credentials.User || p != credentials.Password {
			t.Error("missing private API auth")
		}
		switch mode.Load().(string) {
		case "redirect":
			http.Redirect(w, r, "http://127.0.0.1:1/leak", http.StatusFound)
			return
		case "large":
			w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
			return
		case "wait":
			<-r.Context().Done()
			return
		}
		switch r.URL.Path {
		case "/api/v1/messages":
			if r.Method == "DELETE" {
				var body struct{ IDs []string }
				json.NewDecoder(r.Body).Decode(&body)
				if strings.Join(body.IDs, ",") != "exact" {
					t.Error("unexpected deletion body")
				}
				w.Write([]byte(`{}`))
				return
			}
			w.Write([]byte(`{"total":0,"start":0,"messages":[]}`))
		case "/api/v1/message/exact/raw":
			w.Write([]byte("Subject: plain\r\n\r\nbody"))
		default:
			t.Error("client called endpoint outside allowlist", r.URL.Path)
			http.Error(w, "bad", 400)
		}
	}))
	defer server.Close()
	_, portString, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(portString)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	client, e := NewClient(port, credentials)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	ctx := context.Background()
	if _, e = client.List(ctx, 0); e != nil {
		t.Fatal(e)
	}
	if _, e = client.Raw(ctx, "exact"); e != nil {
		t.Fatal(e)
	}
	if e = client.Delete(ctx, []string{"exact"}); e != nil {
		t.Fatal(e)
	}
	before := calls.Load()
	for _, id := range []string{"latest", "../x", "x/raw?endpoint=release"} {
		if _, e = client.Raw(ctx, id); e == nil {
			t.Fatal("unsafe ID accepted")
		}
		if e = client.Delete(ctx, []string{id}); e == nil {
			t.Fatal("unsafe delete ID")
		}
	}
	if calls.Load() != before {
		t.Fatal("invalid ID reached provider")
	}
	mode.Store("redirect")
	if _, e = client.List(ctx, 0); e == nil {
		t.Fatal("redirect followed")
	}
	mode.Store("large")
	if _, e = client.List(ctx, 0); e == nil {
		t.Fatal("oversize response accepted")
	}
	mode.Store("wait")
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, e = client.List(short, 0); e == nil {
		t.Fatal("deadline ignored")
	}
	file, e := os.Open("/dev/null")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	t.Setenv("MP_SMTP_RELAY_ALL", "true")
	t.Setenv("MP_WEBHOOK_URL", "http://127.0.0.1:1/leak")
	command := Command(file, file, domain.ExecutableSelection{Path: "/not/shell-code"}, Policy{SMTPPort: 1025, APIPort: 8026, MaxMessages: 500, MaxDays: 7}, credentials, "/private/runtime")
	for _, value := range command.Env {
		if strings.HasPrefix(value, "MP_") && !strings.HasPrefix(value, "MP_UI_AUTH=") {
			t.Fatal("inherited unsafe provider setting")
		}
		if strings.Contains(value, "PROXY=") {
			t.Fatal("inherited proxy")
		}
	}
	for _, arg := range command.Args {
		if strings.Contains(arg, credentials.Password) {
			t.Fatal("secret exposed in argv")
		}
	}
	if command.Args[2] != `umask 077; exec /proc/self/fd/3 "$@"` {
		t.Fatal("unexpected launcher code")
	}
}
