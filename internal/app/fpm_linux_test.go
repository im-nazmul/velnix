//go:build linux

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"velnix/internal/process"
	"velnix/internal/state"
)

func startPool(t *testing.T, s HTTPService, req HTTPRequest) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	p, e := s.Plan(ctx, req)
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	req.Approval = p.ID
	done := make(chan error, 1)
	go func() { _, e := s.Apply(ctx, req); done <- e }()
	cleanup := func() {
		cancel()
		select {
		case e := <-done:
			if e != nil && e != context.Canceled {
				t.Errorf("workload exit: %v", e)
			}
		case <-time.After(8 * time.Second):
			t.Error("workload did not exit")
		}
	}
	t.Cleanup(cleanup)
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		select {
		case e := <-done:
			cancel()
			t.Fatalf("workload failed early: %v", e)
		default:
		}
		st, e := s.Status()
		if e == nil && st.Record != nil && st.Record.Phase == "running" && st.Alive {
			return cancel
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("workload never became ready")
	return cancel
}
func TestFPMRealTwoVersionsAndRouting(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	for _, path := range []string{"/usr/sbin/nginx", "/usr/sbin/php-fpm8.3", "/usr/sbin/php-fpm8.5", "/usr/bin/php8.3", "/usr/bin/php8.5"} {
		if _, e := os.Stat(path); e != nil {
			t.Skip("two installed PHP versions and Nginx required")
		}
	}
	s := httpFixture(t)
	st, _ := s.Store.Status()
	for i, p := range st.Registry.Projects {
		version, exe := "8.3.35", "/usr/sbin/php-fpm8.3"
		if i == 1 {
			version, exe = "8.5.11", "/usr/sbin/php-fpm8.5"
		}
		// Derive the locally installed exact version without assuming the recorded one.
		banner, e := exec.Command(exe, "-n", "-v").Output()
		if e != nil {
			t.Fatal(e)
		}
		version = strings.Fields(string(banner))[1]
		script := `<?php echo PHP_VERSION, ' ', PHP_SAPI, ' ini=', (php_ini_loaded_file() ?: 'none'), ' ext=', implode(',',get_loaded_extensions()), ' method=', $_SERVER['REQUEST_METHOD'], ' query=', $_SERVER['QUERY_STRING'], ' uri=', $_SERVER['REQUEST_URI'];`
		if e = os.WriteFile(filepath.Join(p.DocumentRoot, "index.php"), []byte(script), 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.Mkdir(filepath.Join(p.DocumentRoot, "files"), 0700); e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(p.DocumentRoot, "files", "evil.php"), []byte("<?php echo 'bad';"), 0600)
		os.Symlink(filepath.Join(p.DocumentRoot, "index.php"), filepath.Join(p.DocumentRoot, "alias.php"))
		profile := "php"
		if i == 1 {
			profile = "front-controller"
		}
		startPool(t, HTTPService{Store: s.Store, FPMName: p.Name}, HTTPRequest{Action: "start", Executable: exe, Version: version, Profile: profile, Uploads: []string{"files"}})
		// CLI selection remains independent, and no FPM metadata enters the registry.
		cli := strings.Replace(exe, "/usr/sbin/php-fpm", "/usr/bin/php", 1)
		service := RuntimeService{Store: s.Store}
		preview, e := service.Run(context.Background(), RuntimeRequest{Action: "select", Name: p.Name, Path: cli, Version: version})
		if e != nil {
			t.Fatal(e)
		}
		reqCLI := RuntimeRequest{Action: "select", Name: p.Name, Path: cli, Version: version, Approval: preview.(state.ProjectPlan).ID}
		if _, e = service.Run(context.Background(), reqCLI); e != nil {
			t.Fatal(e)
		}
		var output bytes.Buffer
		if e = service.Exec(context.Background(), p.Name, []string{"-r", `echo json_encode(["version"=>PHP_VERSION,"sapi"=>PHP_SAPI,"ini"=>php_ini_loaded_file(),"extensions"=>get_loaded_extensions()]);`}, nil, &output, &output); e != nil {
			t.Fatalf("CLI execution: %q %v", output.String(), e)
		}
		var observed struct {
			Version, SAPI, Ini string
			Extensions         []string
		}
		if e = json.Unmarshal(output.Bytes(), &observed); e != nil || observed.Version != version || observed.SAPI != "cli" || !strings.Contains(observed.Ini, "/cli/php.ini") || len(observed.Extensions) == 0 {
			t.Fatalf("CLI inspection: %s %v", output.String(), e)
		}
		t.Logf("%s CLI: %s", p.Name, output.String())
	}
	port := freePort(t)
	startPool(t, s, HTTPRequest{Action: "start", Executable: "/usr/sbin/nginx", Port: port})
	for _, p := range st.Registry.Projects {
		for _, path := range []string{"/index.php", "/"} {
			code, body, e := requestHTTP(port, p.Domain, path)
			t.Logf("%s%s: %s", p.Domain, path, body)
			expected := "8.3."
			if p.Name == "two" {
				expected = "8.5."
			}
			if e != nil || code != 200 || !strings.HasPrefix(body, expected) || !strings.Contains(body, "fpm-fcgi ini=/etc/php/") || !strings.Contains(body, " ext=") {
				t.Fatalf("%s%s: %d %q %v", p.Domain, path, code, body, e)
			}
		}
		for _, path := range []string{"/.env", "/files/evil.php", "/uploads/evil.php", "/alias.php", "/missing.php", "/index.php/extra"} {
			code, body, e := requestHTTP(port, p.Domain, path)
			if e != nil || (code != 404 && code != 403) {
				t.Fatalf("unsafe response %s: %d %q %v", path, code, body, e)
			}
		}
	}
	code, body, e := requestHTTP(port, "two.test", "/article?q=value")
	if e != nil || code != 200 || !strings.Contains(body, "query=q=value uri=/article?q=value") {
		t.Fatalf("front controller: %d %q %v", code, body, e)
	}
	client := http.Client{Timeout: time.Second}
	request, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/index.php", port), strings.NewReader("value=1"))
	request.Host = "one.test"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, e := client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	postBody, _ := io.ReadAll(response.Body)
	response.Body.Close()
	client.CloseIdleConnections()
	if response.StatusCode != 200 || !strings.Contains(string(postBody), "method=POST") {
		t.Fatalf("PHP POST: %d %s", response.StatusCode, postBody)
	}
	// A dead pool fails visibly and prevents an HTTP reload from blessing it.
	pool := HTTPService{Store: s.Store, FPMName: "one"}
	req := HTTPRequest{Action: "stop"}
	p, e := pool.Plan(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	if _, e = pool.Apply(context.Background(), req); e != nil {
		t.Fatal(e)
	}
	code, _, e = requestHTTP(port, "one.test", "/index.php")
	if e != nil || code != 502 {
		t.Fatalf("dead pool: %d %v", code, e)
	}
	if _, e = s.Plan(context.Background(), HTTPRequest{Action: "reload"}); e == nil {
		t.Fatal("reload accepted stopped pool")
	}
	if code, body, e := requestHTTP(port, "two.test", "/index.php"); e != nil || code != 200 || !strings.HasPrefix(body, "8.5.") {
		t.Fatalf("other pool changed: %d %q %v", code, body, e)
	}
	disable := HTTPRequest{Action: "disable"}
	dp, e := pool.Plan(context.Background(), disable)
	if e != nil {
		t.Fatal(e)
	}
	disable.Approval = dp.ID
	if _, e = pool.Apply(context.Background(), disable); e != nil {
		t.Fatal(e)
	}
	reload := HTTPRequest{Action: "reload"}
	rp, e := s.Plan(context.Background(), reload)
	if e != nil {
		t.Fatal(e)
	}
	reload.Approval = rp.ID
	if _, e = s.Apply(context.Background(), reload); e != nil {
		t.Fatal(e)
	}
	if code, _, e = requestHTTP(port, "one.test", "/index.php"); e != nil || code != 404 {
		t.Fatalf("disabled PHP: %d %v", code, e)
	}
}

func TestFPMPreviewAndGuards(t *testing.T) {
	s := httpFixture(t)
	pool := HTTPService{Store: s.Store, FPMName: "one"}
	for _, r := range []HTTPRequest{{Action: "start", Executable: "/missing", Version: "8.3.1"}, {Action: "start", Executable: "/missing-script", Version: "8.3.1", Profile: "php"}, {Action: "start", Executable: "/usr/sbin/php-fpm8.3", Version: "8.3.35", Profile: "bad"}, {Action: "start", Executable: "/usr/sbin/php-fpm8.3", Version: "8.3.35", Uploads: []string{"../escape"}}} {
		if _, e := pool.Plan(context.Background(), r); e == nil {
			t.Fatal(fmt.Sprintf("accepted %+v", r))
		}
	}
	if b, _ := pool.read(); b != nil {
		t.Fatal("preview created pool evidence")
	}
	if _, e := (HTTPService{Store: s.Store, FPMName: "../../escape"}).Status(); e == nil {
		t.Fatal("accepted unsafe name")
	}
}

func TestFPMFailedProbeAndOccupiedSocket(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	exe := "/usr/sbin/php-fpm8.3"
	if _, e := os.Stat(exe); e != nil {
		t.Skip("installed FPM required")
	}
	s := httpFixture(t)
	pool := HTTPService{Store: s.Store, FPMName: "one"}
	req := HTTPRequest{Action: "start", Executable: exe, Version: "0.0.0", Profile: "php"}
	p, e := pool.Plan(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	if _, e = pool.Apply(context.Background(), req); e == nil {
		t.Fatal("wrong FPM version accepted")
	}
	if b, _ := pool.read(); b != nil {
		t.Fatal("failed probe published running state")
	}
	// A private occupied Unix socket is preserved, even when owned by this UID.
	path := filepath.Join(s.Store.Paths.State, "fpm-one.sock")
	listener, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	banner, _ := exec.Command(exe, "-n", "-v").Output()
	req.Version = strings.Fields(string(banner))[1]
	if _, e = pool.Plan(context.Background(), req); e == nil {
		t.Fatal("occupied socket accepted")
	}
	if _, e = os.Lstat(path); e != nil {
		t.Fatal("occupied socket removed")
	}
}

func TestFPMKilledMasterRecovery(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unprivileged user required")
	}
	exe := "/usr/sbin/php-fpm8.3"
	banner, e := exec.Command(exe, "-n", "-v").Output()
	if e != nil {
		t.Skip("installed FPM required")
	}
	s := httpFixture(t)
	pool := HTTPService{Store: s.Store, FPMName: "one"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := HTTPRequest{Action: "start", Executable: exe, Version: strings.Fields(string(banner))[1], Profile: "php"}
	p, e := pool.Plan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = p.ID
	done := make(chan error, 1)
	go func() { _, e := pool.Apply(ctx, req); done <- e }()
	waitRunning(t, pool)
	st, e := pool.Status()
	if e != nil {
		t.Fatal(e)
	}
	if st.Record.SocketInode == 0 || len(st.Record.Workers) == 0 {
		t.Fatal("missing socket/worker identity")
	}
	if e = process.Signal(st.Record.Process, unix.SIGKILL); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		if e == nil {
			t.Fatal("unexpected death reported success")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("owner did not clean up")
	}
	for _, worker := range st.Record.Workers {
		if process.Alive(worker) {
			t.Fatal("orphaned pool worker")
		}
	}
	recovery := HTTPRequest{Action: "recover"}
	p, e = pool.Plan(context.Background(), recovery)
	if e != nil {
		t.Fatal(e)
	}
	recovery.Approval = p.ID
	if _, e = pool.Apply(context.Background(), recovery); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Lstat(st.Record.FPM.Socket); !os.IsNotExist(e) {
		t.Fatalf("stale owned socket: %v", e)
	}
	// A fresh start can reuse the pool after explicit recovery.
	startPool(t, pool, req)
}
