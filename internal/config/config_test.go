package config

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"defaults", "", true}, {"valid", "schema=1\n[diagnostics]\ntimeout_ms=5000", true},
		{"future", "schema=2", false}, {"unknown", "password='do-not-print'", false},
		{"bad-timeout", "[diagnostics]\ntimeout_ms=0", false}, {"syntax", "[", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.toml")
			os.WriteFile(p, []byte(tc.body), 0600)
			got, err := Load(p, true)
			if (err == nil) != tc.valid {
				t.Fatalf("got %v", err)
			}
			if tc.valid && !got.Exists {
				t.Fatal("missing exists")
			}
		})
	}
}
func TestAbsentAndExplicitMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing")
	r, err := Load(p, false)
	if err != nil || r.Exists || r.Config != Default() {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err = Load(p, true); err == nil {
		t.Fatal("explicit missing config accepted")
	}
}
func TestOversizeAndSymlink(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	os.WriteFile(p, make([]byte, MaxBytes+1), 0600)
	if _, err := Load(p, true); err == nil {
		t.Fatal("oversized input accepted")
	}
	link := p + "-link"
	os.Symlink(p, link)
	if _, err := Load(link, true); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestHostileNesting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	body := "schema=" + strings.Repeat("[", 16000) + "1" + strings.Repeat("]", 16000)
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p, true); err == nil {
		t.Fatal("hostile nested input accepted")
	}
}
func TestFIFOIsRejectedWithoutWaitingForWriter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := syscall.Mkfifo(p, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p, true); err == nil {
		t.Fatal("FIFO accepted")
	}
}
