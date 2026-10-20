//go:build linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"velnix/internal/providers/managedstore"
	"velnix/internal/state"
)

func providerCopy(t *testing.T, source, target string) {
	t.Helper()
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(target, b, 0700); e != nil {
		t.Fatal(e)
	}
}
func TestRealProviderStoreSelection(t *testing.T) {
	for _, kind := range []string{"node", "bun", "python"} {
		t.Run(kind, func(t *testing.T) {
			native, version := installed(t, kind)
			root := t.TempDir()
			provider := "mise"
			entry := filepath.Join("installs", kind, version, "bin", kind)
			if kind == "python" {
				provider = "uv"
				arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
				if arch == "" {
					t.Skip("unsupported uv architecture")
				}
				parts := strings.Split(version, ".")
				entry = filepath.Join("cpython-"+version+"-linux-"+arch+"-gnu", "bin", "python"+parts[0]+"."+parts[1])
			}
			candidate := filepath.Join(root, entry)
			providerCopy(t, native, candidate)
			// A fake manager on PATH and executable-looking manager configuration must
			// remain untouched: filesystem resolution cannot run them.
			marker := filepath.Join(root, "manager-ran")
			bin := filepath.Join(root, "manager-bin")
			os.Mkdir(bin, 0700)
			os.WriteFile(filepath.Join(bin, provider), []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 99\n"), 0700)
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			os.WriteFile(filepath.Join(root, "mise.toml"), []byte("[hooks]\nenter = 'touch "+marker+"'\n"), 0600)
			http := httpFixture(t)
			s := RuntimeService{Store: http.Store}
			locate := RuntimeRequest{Action: "locate", Kind: kind, Provider: provider, ProviderRoot: root, Version: version}
			v, e := s.Run(context.Background(), locate)
			if e != nil {
				t.Fatal(e)
			}
			resolved := v.(managedstore.Resolution)
			if resolved.Probed || resolved.Executable.Path != candidate || resolved.Origin != "unknown" {
				t.Fatalf("resolution %+v", resolved)
			}
			req := locate
			req.Action = "select"
			req.Name = "one"
			value, e := s.Run(context.Background(), req)
			if e != nil {
				t.Fatal(e)
			}
			plan := value.(state.ProjectPlan)
			if plan.Provider == nil || plan.Provider.Provider != provider || plan.Provider.Layout != managedstore.Layout {
				t.Fatal("plan omitted adapter evidence")
			}
			if st, _ := http.Store.Status(); st.Registry.Schema != 2 {
				t.Fatal("preview mutated registry")
			}
			req.Approval = plan.ID
			if _, e = s.Run(context.Background(), req); e != nil {
				t.Fatal(e)
			}
			spec, e := s.ResolveKind("one", kind)
			if e != nil || spec.Executable.Path != candidate || spec.Executable.Version != version {
				t.Fatalf("selected %+v %v", spec, e)
			}
			if _, e = os.Stat(marker); !os.IsNotExist(e) {
				t.Fatal("provider command/hook was executed")
			}
			st, _ := http.Store.Status()
			if len(st.Registry.Resources) != 0 || st.Registry.Schema != 4 {
				t.Fatal("provider selection granted ownership or changed schema contract")
			}
			// Removal/replacement of externally managed files invalidates resolution.
			os.Remove(candidate)
			if _, e = s.ResolveKind("one", kind); e == nil {
				t.Fatal("missing provider executable resolved")
			}
		})
	}
}
func TestProviderSelectionApprovalAndMismatch(t *testing.T) {
	native, version := installed(t, "python")
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch == "" {
		t.Skip("unsupported architecture")
	}
	parts := strings.Split(version, ".")
	root := t.TempDir()
	candidate := filepath.Join(root, "cpython-"+version+"-linux-"+arch+"-gnu", "bin", "python"+parts[0]+"."+parts[1])
	providerCopy(t, native, candidate)
	http := httpFixture(t)
	service := RuntimeService{Store: http.Store}
	req := RuntimeRequest{Action: "select", Name: "one", Kind: "python", Provider: "uv", ProviderRoot: root, Version: version}
	v, e := service.Run(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = v.(state.ProjectPlan).ID
	before, _ := os.ReadFile(candidate)
	b, _ := os.ReadFile("/bin/true")
	os.WriteFile(candidate, b, 0700)
	if _, e = service.Run(context.Background(), req); e == nil {
		t.Fatal("replaced provider executable accepted old approval")
	}
	req.Approval = ""
	v, e = service.Run(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	req.Approval = v.(state.ProjectPlan).ID
	if _, e = service.Run(context.Background(), req); e == nil {
		t.Fatal("filename version accepted without actual version match")
	}
	st, _ := http.Store.Status()
	if st.Registry.Schema != 2 {
		t.Fatal("failed probe committed provider selection")
	}
	os.WriteFile(candidate, before, 0700)
	req.Path = native
	if _, e = service.Run(context.Background(), req); e == nil {
		t.Fatal("provider and executable overrides combined")
	}
}
