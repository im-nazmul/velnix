package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"velnix/internal/app"
	"velnix/internal/config"
	"velnix/internal/domain"
	"velnix/internal/platform"
	"velnix/internal/state"
)

const help = `Velnix — native Linux development environment manager

Usage:
  velnix version [--json]
  velnix doctor [--json] [--config PATH]
  velnix config show|validate [--json] [--config PATH]
  velnix state status [--json]
  velnix state init|recover [--dry-run] [--json]
  velnix state init|recover --approve PLAN_ID [--json]
  velnix http status [--json]
  velnix http start [--nginx /usr/sbin/nginx] [--port 8080] [--dry-run|--approve PLAN_ID] [--json]
  velnix http reload|stop|recover [--dry-run|--approve PLAN_ID] [--json]
  velnix projects list [--json]
  velnix projects inspect --name NAME [--json]
  velnix projects link --name NAME --path PATH [--document-root PATH] [--dry-run|--approve PLAN_ID] [--json]
  velnix projects unlink --name NAME [--dry-run|--approve PLAN_ID] [--json]

State mutations default to a read-only plan. Review its ID, then explicitly approve.
No packages, services, hosts files or trust stores are changed by these commands.
`

type CLI struct {
	Out, Err io.Writer
	Paths    platform.Paths
	Detector app.PlatformReader
	Version  string
}

func (c CLI) Run(ctx context.Context, args []string) int {
	jsonMode := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		switch arg {
		case "--json", "--json=true":
			jsonMode = true
		case "--json=false":
			jsonMode = false
		}
	}
	err := c.run(ctx, args, jsonMode)
	if err == nil {
		return 0
	}
	code := domain.Failure
	var de *domain.Error
	if errors.As(err, &de) {
		code = de.Code
	}
	if errors.Is(err, context.Canceled) {
		code = domain.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		code = domain.Unavailable
	}
	if jsonMode {
		_ = json.NewEncoder(c.Err).Encode(map[string]any{"error": domain.Error{Code: code, Message: err.Error()}})
	} else {
		fmt.Fprintf(c.Err, "velnix: %s\n", err)
	}
	return domain.ExitCode(code)
}
func (c CLI) run(ctx context.Context, args []string, jsonMode bool) error {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")) {
		_, err := io.WriteString(c.Out, help)
		return err
	}
	command := args[0]
	rest := args[1:]
	sub := ""
	if command == "config" || command == "state" || command == "projects" || command == "http" {
		if len(rest) == 0 {
			return domain.E(domain.Invalid, "%s requires a subcommand", command)
		}
		sub = rest[0]
		rest = rest[1:]
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonFlag, dryRun bool
	var cfgPath, approval string
	var projectName, projectPath, documentRoot string
	var nginxPath string
	var httpPort int
	if command == "http" && sub != "status" {
		fs.BoolVar(&dryRun, "dry-run", false, "preview only")
		fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		if sub == "start" {
			fs.StringVar(&nginxPath, "nginx", "/usr/sbin/nginx", "absolute Nginx executable")
			fs.IntVar(&httpPort, "port", 8080, "loopback HTTP port")
		}
	}
	fs.BoolVar(&jsonFlag, "json", false, "structured output")
	if command == "doctor" || command == "config" {
		fs.StringVar(&cfgPath, "config", "", "configuration path")
	}
	if command == "state" && (sub == "init" || sub == "recover") {
		fs.BoolVar(&dryRun, "dry-run", false, "preview only")
		fs.StringVar(&approval, "approve", "", "reviewed plan ID")
	}
	if command == "projects" {
		if sub == "inspect" || sub == "link" || sub == "unlink" {
			fs.StringVar(&projectName, "name", "", "project name")
		}
		if sub == "link" {
			fs.StringVar(&projectPath, "path", ".", "project directory")
			fs.StringVar(&documentRoot, "document-root", ".", "document root relative to the project directory")
		}
		if sub == "link" || sub == "unlink" {
			fs.BoolVar(&dryRun, "dry-run", false, "preview only")
			fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		}
	}
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, e := io.WriteString(c.Out, help)
			return e
		}
		return domain.E(domain.Invalid, "%v", err)
	}
	jsonMode = jsonFlag
	if fs.NArg() != 0 {
		return domain.E(domain.Invalid, "unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	output := func(v any, text string) error {
		if jsonMode {
			return json.NewEncoder(c.Out).Encode(v)
		}
		_, err := fmt.Fprintln(c.Out, text)
		return err
	}
	if command == "version" {
		return output(map[string]string{"version": c.Version}, "velnix "+c.Version)
	}
	if c.Paths.Config == "" {
		var err error
		c.Paths, err = platform.UserPaths()
		if err != nil {
			return err
		}
	}
	if command == "state" {
		service := app.StateService{Store: state.Store{Paths: c.Paths}}
		result, err := service.Run(ctx, app.StateRequest{Action: sub, DryRun: dryRun, ApprovedPlanID: approval})
		if err != nil {
			return err
		}
		var value any = result.Status
		if result.Plan != nil {
			value = result.Plan
		}
		b, _ := json.MarshalIndent(value, "", "  ")
		return output(value, string(b))
	}
	if command == "http" {
		service := app.HTTPService{Store: state.Store{Paths: c.Paths}}
		req := app.HTTPRequest{Action: sub, Executable: nginxPath, Port: httpPort, Approval: approval, DryRun: dryRun}
		var value any
		var err error
		if sub == "status" {
			value, err = service.Status()
		} else if dryRun && approval != "" {
			return domain.E(domain.Invalid, "--dry-run cannot be combined with --approve")
		} else if approval == "" {
			value, err = service.Plan(ctx, req)
		} else {
			value, err = service.Apply(ctx, req)
		}
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return output(value, string(b))
	}
	if command == "projects" {
		service := app.ProjectService{Store: state.Store{Paths: c.Paths}}
		value, err := service.Run(ctx, app.ProjectRequest{ProjectChange: state.ProjectChange{Action: sub, Name: projectName, Root: projectPath, DocumentRoot: documentRoot}, DryRun: dryRun, ApprovedPlanID: approval})
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return output(value, string(b))
	}

	if command != "doctor" && command != "config" {
		return domain.E(domain.Invalid, "unknown command %q; run velnix --help", command)
	}
	if command == "config" && sub != "show" && sub != "validate" {
		return domain.E(domain.Invalid, "unknown config subcommand %q", sub)
	}
	required := cfgPath != ""
	if !required {
		cfgPath = filepath.Join(c.Paths.Config, "config.toml")
	}
	cfg, err := config.Load(cfgPath, required)
	if err != nil {
		return err
	}
	if command == "config" {
		b, _ := json.MarshalIndent(cfg, "", "  ")
		return output(cfg, string(b))
	}
	doctorCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Config.Diagnostics.TimeoutMS)*time.Millisecond)
	defer cancel()
	report, err := (app.Doctor{Platform: c.Detector, State: state.Store{Paths: c.Paths}, Paths: c.Paths}).Run(doctorCtx, cfg)
	if err != nil {
		return err
	}
	var text strings.Builder
	fmt.Fprintf(&text, "Platform: %s %s (%s), WSL: %t\n", report.Platform.Distribution, report.Platform.DistributionVersion, report.Platform.Architecture, report.Platform.WSL)
	for _, check := range report.Checks {
		fmt.Fprintf(&text, "[%s] %s: %s\n", check.Status, check.Name, check.Detail)
	}
	if err = output(report, strings.TrimSpace(text.String())); err != nil {
		return err
	}
	if !report.Healthy {
		return domain.E(domain.Unavailable, "doctor found errors; inspect the checks above")
	}
	return nil
}
