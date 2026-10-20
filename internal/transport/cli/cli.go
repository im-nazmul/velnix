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
  velnix fpm start --name NAME --php-fpm /absolute/php-fpm --version X.Y.Z [--profile php|front-controller] [--uploads uploads,files] [--dry-run|--approve PLAN_ID] [--json]
  velnix fpm status|stop|recover|disable --name NAME [--dry-run|--approve PLAN_ID] [--json]
  velnix tls issue [--mkcert /absolute/mkcert] [--dry-run|--approve PLAN_ID] [--json]
  velnix tls status|export-ca [--json]
  velnix tls recover [--dry-run|--approve PLAN_ID] [--json]
  velnix tls verify --name NAME [--port 8443] [--json]
  velnix http status [--json]
  velnix http start [--nginx /usr/sbin/nginx] [--port 8080] [--https] [--https-port 8443] [--dry-run|--approve PLAN_ID] [--json]
  velnix http reload|stop|recover [--dry-run|--approve PLAN_ID] [--json]
  velnix projects list [--json]
  velnix projects inspect|detect --name NAME [--json]
  velnix projects proxy --name NAME --port PORT [--dry-run|--approve PLAN_ID] [--json]
  velnix projects link --name NAME --path PATH [--document-root PATH] [--dry-run|--approve PLAN_ID] [--json]
  velnix projects unlink --name NAME [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime providers [--json]
  velnix runtime locate --provider mise|uv --provider-root /absolute/store --kind node|bun|python --version X.Y.Z [--json]
  velnix runtime select --provider mise|uv --provider-root /absolute/store --kind node|bun|python --name NAME --version X.Y.Z [--venv /absolute/.venv] [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime select --kind node|bun|python --name NAME --executable /absolute/binary --version X.Y.Z [--venv /absolute/.venv] [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime clear --kind node|bun|python --name NAME [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime resolve --kind node|bun|python --name NAME [--json]
  velnix runtime list [--kind php|node|bun|python] [--directory /absolute/path] [--json]
  velnix runtime inspect --php /absolute/php --version X.Y.Z [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime inspect --kind node|bun|python --executable /absolute/binary --version X.Y.Z [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime select --name NAME --php /absolute/php --version X.Y.Z [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime clear --name NAME [--dry-run|--approve PLAN_ID] [--json]
  velnix runtime resolve --name NAME [--json]
  velnix exec --name NAME [--kind php|node|bun|python] -- RUNTIME_ARGUMENTS...

State mutations default to a read-only plan. Review its ID, then explicitly approve.
No packages, services, hosts files or trust stores are changed by these commands.
`

type CLI struct {
	In       io.Reader
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
	var child *app.ChildExit
	if errors.As(err, &child) {
		return child.Code
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
	var phpArgs []string
	if command == "exec" {
		separator := -1
		for i, arg := range rest {
			if arg == "--" {
				separator = i
				break
			}
		}
		if separator < 0 {
			return domain.E(domain.Invalid, "exec requires -- before runtime arguments")
		}
		phpArgs, rest = rest[separator+1:], rest[:separator]
	}
	sub := ""
	if command == "config" || command == "state" || command == "projects" || command == "http" || command == "fpm" || command == "tls" || command == "runtime" {
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
	var phpPath, phpVersion string
	var runtimeKind, runtimeDirectory string
	var runtimeExecutable string
	var runtimeVenv string
	var runtimeProvider, providerRoot string
	var proxyPort int
	var httpPort int
	var tlsExecutable string
	var https bool
	var httpsPort int
	if command == "tls" {
		if sub == "issue" || sub == "recover" {
			fs.BoolVar(&dryRun, "dry-run", false, "preview only")
			fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		}
		if sub == "issue" {
			fs.StringVar(&tlsExecutable, "mkcert", "/usr/local/bin/mkcert", "absolute installed mkcert v1.4.4 executable")
		}
		if sub == "verify" {
			fs.StringVar(&projectName, "name", "", "registered project name")
			fs.IntVar(&httpsPort, "port", 8443, "loopback HTTPS port")
		}
	}
	var fpmProfile, uploadDirectories string
	if command == "fpm" {
		fs.StringVar(&projectName, "name", "", "linked project name")
		if sub != "status" {
			fs.BoolVar(&dryRun, "dry-run", false, "preview only")
			fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		}
		if sub == "start" {
			fs.StringVar(&phpPath, "php-fpm", "", "absolute native FPM executable")
			fs.StringVar(&phpVersion, "version", "", "exact FPM version")
			fs.StringVar(&fpmProfile, "profile", "php", "php or front-controller")
			fs.StringVar(&uploadDirectories, "uploads", "uploads", "comma-separated relative upload directories denied by web routing")
		}
	}
	if command == "http" && sub != "status" {
		fs.BoolVar(&dryRun, "dry-run", false, "preview only")
		fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		if sub == "start" {
			fs.StringVar(&nginxPath, "nginx", "/usr/sbin/nginx", "absolute Nginx executable")
			fs.IntVar(&httpPort, "port", 8080, "loopback HTTP port")
			fs.BoolVar(&https, "https", false, "enable HTTPS with current issued certificate")
			fs.IntVar(&httpsPort, "https-port", 8443, "loopback HTTPS port")
		}
	}
	if command != "exec" {
		fs.BoolVar(&jsonFlag, "json", false, "structured output")
	}
	if command == "runtime" && sub == "list" {
		fs.StringVar(&runtimeKind, "kind", "php", "runtime candidate kind")
		fs.StringVar(&runtimeDirectory, "directory", "", "explicit discovery directory for Node, Bun or Python")
	}
	if (command == "runtime" && (sub == "locate" || sub == "inspect" || sub == "select" || sub == "clear" || sub == "resolve")) || command == "exec" {
		fs.StringVar(&runtimeKind, "kind", "php", "runtime kind")
		if command == "runtime" && (sub == "inspect" || sub == "select") {
			fs.StringVar(&runtimeExecutable, "executable", "", "absolute existing Node, Bun or Python executable")
		}
		if sub == "select" {
			fs.StringVar(&runtimeVenv, "venv", "", "existing Python virtualenv inside project")
		}
	}
	if command == "exec" || (command == "runtime" && (sub == "select" || sub == "clear" || sub == "resolve")) {
		fs.StringVar(&projectName, "name", "", "linked project name")
	}
	if command == "runtime" && (sub == "locate" || sub == "select") {
		fs.StringVar(&runtimeProvider, "provider", "", "read-only existing provider store: mise or uv")
		fs.StringVar(&providerRoot, "provider-root", "", "explicit existing provider storage root")
		if sub == "locate" {
			fs.StringVar(&phpVersion, "version", "", "exact installed release version")
		}
	}
	if command == "runtime" && (sub == "inspect" || sub == "select" || sub == "clear") {
		fs.BoolVar(&dryRun, "dry-run", false, "preview only")
		fs.StringVar(&approval, "approve", "", "reviewed plan ID")
		if sub != "clear" {
			fs.StringVar(&phpPath, "php", "", "absolute existing PHP CLI executable")
			fs.StringVar(&phpVersion, "version", "", "exact required PHP version")
		}
	}
	if command == "doctor" || command == "config" {
		fs.StringVar(&cfgPath, "config", "", "configuration path")
	}
	if command == "state" && (sub == "init" || sub == "recover") {
		fs.BoolVar(&dryRun, "dry-run", false, "preview only")
		fs.StringVar(&approval, "approve", "", "reviewed plan ID")
	}
	if command == "projects" {
		if sub == "inspect" || sub == "link" || sub == "unlink" || sub == "detect" || sub == "proxy" {
			fs.StringVar(&projectName, "name", "", "project name")
		}
		if sub == "link" {
			fs.StringVar(&projectPath, "path", ".", "project directory")
			fs.StringVar(&documentRoot, "document-root", ".", "document root relative to the project directory")
		}
		if sub == "proxy" {
			fs.IntVar(&proxyPort, "port", 0, "already-running loopback upstream port; zero disables")
		}
		if sub == "link" || sub == "unlink" || sub == "proxy" {
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
	if command == "runtime" {
		if sub == "inspect" || sub == "select" {
			if runtimeKind == "" {
				return domain.E(domain.Invalid, "runtime kind must be php, node, bun or python")
			}
			if runtimeKind == "php" && runtimeExecutable != "" {
				return domain.E(domain.Invalid, "PHP inspection requires --php; --executable requires --kind node, bun or python")
			}
			if runtimeKind != "php" {
				if phpPath != "" {
					return domain.E(domain.Invalid, "--php cannot be combined with a different runtime kind")
				}
				phpPath = runtimeExecutable
			}
		}
		value, err := (app.RuntimeService{Store: state.Store{Paths: c.Paths}}).Run(ctx, app.RuntimeRequest{Action: sub, Name: projectName, Path: phpPath, Version: phpVersion, Approval: approval, DryRun: dryRun, Kind: runtimeKind, Directory: runtimeDirectory, Venv: runtimeVenv, Provider: runtimeProvider, ProviderRoot: providerRoot})
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return output(value, string(b))
	}
	if command == "exec" {
		return (app.RuntimeService{Store: state.Store{Paths: c.Paths}}).ExecKind(ctx, projectName, runtimeKind, phpArgs, c.In, c.Out, c.Err)
	}
	if command == "tls" {
		service := app.TLSService{Store: state.Store{Paths: c.Paths}}
		var value any
		var err error
		if sub == "verify" {
			value, err = service.Verify(ctx, projectName, httpsPort)
		} else {
			value, err = service.Run(ctx, app.TLSRequest{Action: sub, Executable: tlsExecutable, Approval: approval, DryRun: dryRun})
		}
		if err != nil {
			return err
		}
		if sub == "export-ca" && !jsonMode {
			_, err = io.WriteString(c.Out, value.(string))
			return err
		}
		b, _ := json.MarshalIndent(value, "", "  ")
		return output(value, string(b))
	}
	if command == "http" || command == "fpm" {
		service := app.HTTPService{Store: state.Store{Paths: c.Paths}}
		req := app.HTTPRequest{Action: sub, Executable: nginxPath, Port: httpPort, Approval: approval, DryRun: dryRun, HTTPS: https, HTTPSPort: httpsPort}
		if command == "fpm" {
			if !domain.ValidProjectName(projectName) {
				return domain.E(domain.Invalid, "fpm requires a valid --name")
			}
			service.FPMName = projectName
			req.Executable = phpPath
			req.Version = phpVersion
			req.Profile = fpmProfile
			if uploadDirectories != "" {
				req.Uploads = strings.Split(uploadDirectories, ",")
			}
		}
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
		value, err := service.Run(ctx, app.ProjectRequest{ProjectChange: state.ProjectChange{Action: sub, Name: projectName, Root: projectPath, DocumentRoot: documentRoot, ProxyPort: proxyPort}, DryRun: dryRun, ApprovedPlanID: approval})
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
