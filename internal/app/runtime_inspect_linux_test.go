//go:build linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeInspectionApprovalBindsKindVersionAndExecutable(t *testing.T) {
	b, err := os.ReadFile("/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "candidate")
	if err = os.WriteFile(path, b, 0700); err != nil {
		t.Fatal(err)
	}
	service := RuntimeService{} // Inspection must never access persistent state.
	for _, action := range []string{"select", "clear", "resolve"} {
		if _, err := service.Run(context.Background(), RuntimeRequest{Action: action, Kind: "node"}); err == nil {
			t.Fatalf("Node %s without a project name accepted", action)
		}
	}
	req := RuntimeRequest{Action: "inspect", Kind: "node", Path: path, Version: "1.2.3"}
	v, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	plan := v.(RuntimeProbePlan)
	if plan.ID == "" || plan.Kind != "node" || plan.Argv[0] != "--version" {
		t.Fatalf("%+v", plan)
	}
	for _, changed := range []RuntimeRequest{
		{Action: "inspect", Kind: "bun", Path: path, Version: "1.2.3", Approval: plan.ID},
		{Action: "inspect", Kind: "node", Path: path, Version: "1.2.4", Approval: plan.ID},
		{Action: "inspect", Kind: "node", Path: path, Version: "1.2.3", Approval: "incorrect"},
	} {
		if _, err = service.Run(context.Background(), changed); err == nil {
			t.Fatal("stale approval accepted")
		}
	}
	req.Approval = plan.ID
	if _, err = service.Run(context.Background(), req); err == nil {
		t.Fatal("non-runtime accepted as version match")
	}
	req.DryRun = true
	if _, err = service.Run(context.Background(), req); err == nil {
		t.Fatal("dry-run with approval accepted")
	}
	req.DryRun = false
	if err = os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Run(context.Background(), req); err == nil {
		t.Fatal("replacement retained approval")
	}
	req.Approval = ""
	if err = os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Run(context.Background(), req); err == nil {
		t.Fatal("script accepted")
	}
}
