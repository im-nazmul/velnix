package nginx

import (
	"strings"
	"testing"
	"velnix/internal/domain"
)

func TestRenderRejectsConfigInjection(t *testing.T) {
	for _, path := range []string{"/tmp/a;evil", "/tmp/$variable", "/tmp/a\nb", "/tmp/a\"b", "/tmp/a\\b"} {
		if _, e := Render(nil, 8080, "token", path); e == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	if _, e := Render(nil, 80, "token", "/tmp/private"); e == nil {
		t.Fatal("accepted privileged port")
	}
}
func TestRenderIsIsolated(t *testing.T) {
	p := domain.Project{Name: "demo", Domain: "demo.test", Root: "/tmp/a space", DocumentRoot: "/tmp/a space", Inode: 1, DocumentInode: 1}
	b, e := Render([]domain.Project{p}, 8080, "token", "/tmp/private")
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"listen 127.0.0.1:8080", "disable_symlinks on", "access_log off", "root \"/tmp/a space\""} {
		if !strings.Contains(b, want) {
			t.Fatal(want)
		}
	}
	if strings.Contains(b, "include ") {
		t.Fatal("external config included")
	}
}

func TestDiagnosticsAreBounded(t *testing.T) {
	var tail Tail
	input := strings.Repeat("x", 20000)
	if n, e := tail.Write([]byte(input)); e != nil || n != len(input) {
		t.Fatal(n, e)
	}
	if len(tail.String()) != 8192 {
		t.Fatal("unbounded diagnostics")
	}
}
