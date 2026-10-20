package platform

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPaths(t *testing.T) {
	env := map[string]string{"HOME": "/home/test", "XDG_CONFIG_HOME": "relative", "XDG_STATE_HOME": "/state"}
	got, err := ResolvePaths(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if got.Config != "/home/test/.config/velnix" || got.State != "/state/velnix" {
		t.Fatalf("%+v", got)
	}
	env["HOME"] = "relative"
	if _, err = ResolvePaths(func(k string) string { return env[k] }); err == nil {
		t.Fatal("relative HOME accepted")
	}
}
func TestDetectionReadsOnlyMetadata(t *testing.T) {
	root := t.TempDir()
	put := func(p, s string, mode os.FileMode) {
		p = filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := os.WriteFile(p, []byte(s), mode); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(root, "executed")
	put("etc/os-release", "ID=ubuntu\nVERSION_ID=\"24.04\"", 0600)
	put("proc/sys/kernel/osrelease", "6.6-microsoft-standard-WSL2", 0600)
	put("proc/1/comm", "systemd\n", 0600)
	os.MkdirAll(filepath.Join(root, "run/systemd/system"), 0700)
	put("usr/bin/apt-get", "#!/bin/sh\ntouch "+marker, 0700)
	f, err := (Detector{Root: root}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !f.WSL || !f.Systemd || f.Distribution != "ubuntu" || len(f.PackageManagers) != 1 {
		t.Fatalf("%+v", f)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("discovery executed a package manager")
	}
}
func TestMissingMetadataAndCancellation(t *testing.T) {
	d := Detector{Root: t.TempDir()}
	f, err := d.Detect(context.Background())
	if err != nil || f.Systemd || f.WSL || len(f.Warnings) == 0 {
		t.Fatalf("%+v %v", f, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = d.Detect(ctx); err == nil {
		t.Fatal("cancellation ignored")
	}
}
