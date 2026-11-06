//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"velnix/internal/app"
	"velnix/internal/domain"
	"velnix/internal/platform"
	"velnix/internal/transport/web"
)

func (c CLI) ui(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	port := 8090
	dry, jsonMode := false, false
	approval := ""
	fs.IntVar(&port, "port", 8090, "numeric loopback port")
	fs.BoolVar(&dry, "dry-run", false, "preview only")
	fs.BoolVar(&jsonMode, "json", false, "structured output")
	fs.StringVar(&approval, "approve", "", "exact reviewed launch plan ID")
	if e := fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			_, e = io.WriteString(c.Out, "velnix ui [--port 8090] [--dry-run|--approve PLAN_ID] [--json]\nStart the embedded dashboard in the foreground after reviewing its exact plan.\n")
			return e
		}
		return domain.E(domain.Invalid, "%v", e)
	}
	if fs.NArg() != 0 || dry && approval != "" {
		return domain.E(domain.Invalid, "unexpected UI arguments or conflicting approval flags")
	}
	if c.Paths.Config == "" {
		var e error
		c.Paths, e = platform.UserPaths()
		if e != nil {
			return e
		}
	}
	p, e := app.PlanDashboard(c.Paths, port)
	if e != nil {
		return e
	}
	if approval == "" {
		if jsonMode {
			return json.NewEncoder(c.Out).Encode(p)
		}
		b, _ := json.MarshalIndent(p, "", "  ")
		_, e = fmt.Fprintln(c.Out, string(b))
		return e
	}
	if p.ID != approval {
		return domain.E(domain.Conflict, "UI launch plan changed; review again")
	}
	platformReader := c.Detector
	if platformReader == nil {
		platformReader = platform.Detector{Root: "/"}
	}
	return web.Serve(ctx, p, app.Dashboard{Paths: c.Paths, Platform: platformReader}, func(address, bootstrap string) error {
		if jsonMode {
			return json.NewEncoder(c.Out).Encode(map[string]string{"event": "ui_ready", "address": address, "bootstrap_file": bootstrap})
		}
		_, e := fmt.Fprintf(c.Out, "Dashboard: %s\nOne-time login secret: read %s and paste its contents into the login form.\nPress Ctrl-C to stop.\n", address, bootstrap)
		return e
	})
}
