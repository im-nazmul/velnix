package state

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"velnix/internal/platform"
)

func fixture(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	return Store{Paths: platform.Paths{Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}
}
func initStore(t *testing.T, s Store) {
	t.Helper()
	p, err := s.Plan("init")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), "init", p.ID); err != nil {
		t.Fatal(err)
	}
}
func TestPlanReadOnlyAndApproval(t *testing.T) {
	s := fixture(t)
	p, err := s.Plan("init")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.Paths.State, s.Paths.Data} {
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("preview created %s", path)
		}
	}
	if _, err = s.Apply(context.Background(), "init", "bad"); err == nil {
		t.Fatal("wrong approval accepted")
	}
	st, err := s.Apply(context.Background(), "init", p.ID)
	if err != nil || !st.Initialized || st.RecoveryRequired {
		t.Fatalf("%+v %v", st, err)
	}
	for _, path := range []string{s.Paths.State, s.Paths.Data} {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0700 {
			t.Fatal(info.Mode())
		}
	}
	for _, path := range []string{filepath.Join(s.Paths.Data, "registry.json"), filepath.Join(s.Paths.State, "journal.json")} {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
	}
	p, err = s.Plan("init")
	if err != nil || len(p.Effects) != 0 {
		t.Fatalf("init not idempotent: %+v %v", p, err)
	}
}
func TestSchemaPrivacyAndLinks(t *testing.T) {
	for _, kind := range []string{"future", "invalid", "null", "missing-schema", "missing-resources", "trailing", "directory-mode", "file-mode", "symlink", "parent-symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			s := fixture(t)
			initStore(t, s)
			path := filepath.Join(s.Paths.Data, "registry.json")
			switch kind {
			case "future":
				os.WriteFile(path, []byte(`{"schema":2,"resources":[]}`), 0600)
			case "null":
				os.WriteFile(path, []byte("null"), 0600)
			case "missing-schema":
				os.WriteFile(path, []byte(`{"resources":[]}`), 0600)
			case "missing-resources":
				os.WriteFile(path, []byte(`{"schema":1}`), 0600)
			case "invalid":
				os.WriteFile(path, []byte("{"), 0600)
			case "trailing":
				os.WriteFile(path, []byte(`{"schema":1,"resources":[]} {}`), 0600)
			case "directory-mode":
				os.Chmod(s.Paths.Data, 0755)
			case "file-mode":
				os.Chmod(path, 0644)
			case "symlink":
				os.Rename(path, path+".old")
				os.Symlink(path+".old", path)
			case "parent-symlink":
				os.Rename(s.Paths.Data, s.Paths.Data+".old")
				os.Symlink(s.Paths.Data+".old", s.Paths.Data)
			case "hardlink":
				os.Link(path, path+".link")
			}
			if _, err := s.Status(); err == nil {
				t.Fatal("unsafe state accepted")
			}
		})
	}
}
func TestChangedPlanAndConcurrentApply(t *testing.T) {
	s := fixture(t)
	p, _ := s.Plan("init")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Apply(context.Background(), "init", p.ID); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successes", successes)
	}
	if _, err := s.Apply(context.Background(), "init", p.ID); err == nil {
		t.Fatal("stale plan accepted")
	}
}
func TestRecoveryBoundaries(t *testing.T) {
	for _, status := range []string{"planned", "approved", "applying", "verified", "complete"} {
		for _, committed := range []bool{false, true} {
			t.Run(status+map[bool]string{true: "-committed", false: "-absent"}[committed], func(t *testing.T) {
				s := fixture(t)
				os.MkdirAll(s.Paths.State, 0700)
				j := Journal{Schema: 1, Operation: "init", PlanID: digest([]byte("plan")), Status: status, TargetDigest: digest(encoded(empty()))}
				os.WriteFile(filepath.Join(s.Paths.State, "journal.json"), encoded(j), 0600)
				if committed {
					os.MkdirAll(s.Paths.Data, 0700)
					os.WriteFile(filepath.Join(s.Paths.Data, "registry.json"), encoded(empty()), 0600)
				}
				before, err := s.Status()
				if err != nil {
					t.Fatal(err)
				}
				needsRecovery := !committed || status != "complete"
				if before.RecoveryRequired != needsRecovery {
					t.Fatalf("%+v", before)
				}
				p, err := s.Plan("recover")
				if err != nil {
					t.Fatal(err)
				}
				after, err := s.Apply(context.Background(), "recover", p.ID)
				if err != nil || after.RecoveryRequired || after.Initialized != committed {
					t.Fatalf("%+v %v", after, err)
				}
				if !committed {
					if _, err = os.Stat(s.Paths.Data); !os.IsNotExist(err) {
						t.Fatal("recovery replayed registry write")
					}
				}
			})
		}
	}
}
func TestRecoveryPreservesExternalEdits(t *testing.T) {
	s := fixture(t)
	initStore(t, s)
	path := filepath.Join(s.Paths.Data, "registry.json")
	content := []byte("{\"schema\":1,\"resources\":[]}\n") // different digest
	os.WriteFile(path, content, 0600)
	if _, err := s.Plan("recover"); err == nil {
		t.Fatal("changed registry accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != string(content) {
		t.Fatal("external edits changed")
	}
}
func TestLockHelper(t *testing.T) {
	if os.Getenv("VELNIX_TEST_LOCK") == "" {
		return
	}
	d, err := openDir(os.Getenv("VELNIX_TEST_LOCK"), false)
	if err != nil {
		os.Exit(2)
	}
	f, err := d.open("mutation.lock", syscall.O_RDWR|syscall.O_CREAT)
	if err != nil {
		os.Exit(3)
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX) != nil {
		os.Exit(4)
	}
	os.Stdout.Write([]byte("locked\n"))
	for {
		time.Sleep(time.Hour)
	}
}
func TestInterprocessLockCancellationAndCrashRelease(t *testing.T) {
	s := fixture(t)
	os.MkdirAll(s.Paths.State, 0700)
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$")
	cmd.Env = append(os.Environ(), "VELNIX_TEST_LOCK="+s.Paths.State)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ready := make(chan error, 1)
	go func() { buf := make([]byte, 7); _, e := io.ReadFull(pipe, buf); ready <- e }()
	select {
	case err = <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not lock")
	}
	p, err := s.Plan("init")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err = s.Apply(ctx, "init", p.ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
	cmd.Process.Kill()
	cmd.Wait()
	if _, err = s.Apply(context.Background(), "init", p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestInitialPublishNeverReplacesExistingFile(t *testing.T) {
	s := fixture(t)
	d, err := openDir(s.Paths.Data, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	if err = d.publish("registry.json", []byte("external-state"), true); err != nil {
		t.Fatal(err)
	}
	if err = d.publish("registry.json", []byte("replacement"), true); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected exists error, got %v", err)
	}
	b, err := d.read("registry.json")
	if err != nil || string(b) != "external-state" {
		t.Fatalf("%q %v", b, err)
	}
}
