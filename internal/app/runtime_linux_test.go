package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"velnix/internal/domain"
	"velnix/internal/providers/phpcli"
	"velnix/internal/state"
)

func runtimePHP(t *testing.T) (string, string) {
	t.Helper()
	for _, p := range []string{"/usr/local/bin/php", "/usr/bin/php"} {
		if _, err := phpcli.Snapshot(p, "8.3.12"); err != nil {
			continue
		}
		c := exec.Command(p, "-n", "-r", "echo PHP_VERSION;")
		c.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
		b, err := c.Output()
		if err != nil {
			t.Fatal(err)
		}
		return p, string(b)
	}
	t.Skip("native PHP CLI not installed")
	return "", ""
}

func TestRuntimeSelectionResolveExecAndClear(t *testing.T) {
	path, version := runtimePHP(t)
	store := httpFixture(t).Store
	s := RuntimeService{Store: store}
	req := RuntimeRequest{Action: "select", Name: "one", Path: path, Version: version}
	if _, err := s.Resolve("one"); err == nil {
		t.Fatal("unselected project resolved")
	}
	v, err := s.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	p := v.(state.ProjectPlan)
	st, _ := store.Status()
	if st.Registry.Schema != 2 || st.Registry.Projects[0].PHPCLI != (domain.PHPSelection{}) {
		t.Fatal("preview mutated registry")
	}
	req.Approval = "bad"
	if _, err = s.Run(context.Background(), req); err == nil {
		t.Fatal("wrong approval accepted")
	}
	req.Approval = p.ID
	if _, err = s.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	st, err = store.Status()
	if err != nil || st.Registry.Schema != 3 || st.RecoveryRequired {
		t.Fatalf("%+v %v", st, err)
	}
	if st.Registry.Projects[1].PHPCLI != (domain.PHPSelection{}) {
		t.Fatal("other project selection changed")
	}
	spec, err := s.Resolve("one")
	if err != nil || spec.Executable.Version != version {
		t.Fatalf("%+v %v", spec, err)
	}
	var out, stderr bytes.Buffer
	arg := "$(touch should-not-exist); literal `echo`"
	err = s.Exec(context.Background(), "one", []string{"-n", "-r", `echo getcwd(), "\n", getenv("VELNIX_PHP"), "\n", $argv[1], "\n", stream_get_contents(STDIN);`, "--", arg}, strings.NewReader("input"), &out, &stderr)
	if err != nil || stderr.Len() != 0 || out.String() != spec.Directory+"\n"+spec.Executable.Path+"\n"+arg+"\ninput" {
		t.Fatalf("%v stdout=%s stderr=%s", err, &out, &stderr)
	}
	err = s.Exec(context.Background(), "one", []string{"-n", "-r", "exit(23);"}, nil, &out, &stderr)
	var child *ChildExit
	if !errors.As(err, &child) || child.Code != 23 {
		t.Fatalf("child status lost: %v", err)
	}
	// Relink remains idempotent and preserves the runtime selection.
	link := state.ProjectChange{Action: "link", Name: "one", Root: spec.Directory, DocumentRoot: "."}
	linkPlan, err := store.PlanProject(link)
	if err != nil || len(linkPlan.Effects) != 0 {
		t.Fatalf("relink lost selection: %+v %v", linkPlan, err)
	}
	clear := RuntimeRequest{Action: "clear", Name: "one"}
	v, err = s.Run(context.Background(), clear)
	if err != nil {
		t.Fatal(err)
	}
	clear.Approval = v.(state.ProjectPlan).ID
	if _, err = s.Run(context.Background(), clear); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve("one"); err == nil {
		t.Fatal("cleared selection still resolved")
	}
	if _, err = os.Stat(spec.Directory); err != nil {
		t.Fatal("clear removed source")
	}
}

func TestRuntimeFailedProbeDoesNotCommitAndChangedBinaryInvalidatesPlan(t *testing.T) {
	store := httpFixture(t).Store
	s := RuntimeService{Store: store}
	path := filepath.Join(t.TempDir(), "php")
	b, err := os.ReadFile("/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0700); err != nil {
		t.Fatal(err)
	}
	req := RuntimeRequest{Action: "select", Name: "one", Path: path, Version: "8.3.12"}
	v, err := s.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	req.Approval = v.(state.ProjectPlan).ID
	if _, err = s.Run(context.Background(), req); err == nil {
		t.Fatal("non-PHP ELF accepted by probe")
	}
	st, err := store.Status()
	if err != nil || st.RecoveryRequired || st.Registry.Schema != 2 {
		t.Fatalf("failed probe committed: %+v %v", st, err)
	}
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Run(context.Background(), req); err == nil {
		t.Fatal("replacement did not invalidate approval")
	}
}

func TestRuntimeExecCancellation(t *testing.T) {
	path, version := runtimePHP(t)
	s := RuntimeService{Store: httpFixture(t).Store}
	req := RuntimeRequest{Action: "select", Name: "one", Path: path, Version: version}
	v, err := s.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	req.Approval = v.(state.ProjectPlan).ID
	if _, err = s.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = s.Exec(ctx, "one", []string{"-n", "-r", "sleep(30);"}, nil, &bytes.Buffer{}, &bytes.Buffer{})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 4*time.Second {
		t.Fatalf("cancellation did not reap PHP: %v", err)
	}
}
