//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"velnix/internal/app"
	"velnix/internal/domain"
	"velnix/internal/platform"
	"velnix/internal/state"
)

const mailHelp = `Usage:
  velnix mail status [--json]
  velnix mail serve [--mailpit /usr/local/bin/mailpit] [--smtp-port 1025] [--api-port 8026] [--max-messages 500] [--max-days 7] [--dry-run|--approve PLAN_ID] [--json]
  velnix mail list [--offset 0] [--json]
  velnix mail show ID [--json]
  velnix mail export ID --attachment ID --output /absolute/file [--dry-run|--approve PLAN_ID] [--json]
  velnix mail prune|recover [--dry-run|--approve PLAN_ID] [--json]

Mail mutations default to a plan. Capture runs in the foreground until Ctrl-C.
SMTP/API bind only to 127.0.0.1. No existing external Mailpit instance is adopted.
`

func (c CLI) mail(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, e := io.WriteString(c.Out, mailHelp)
		return e
	}
	req := app.MailRequest{Action: args[0]}
	args = args[1:]
	switch req.Action {
	case "status", "serve", "list", "show", "export", "prune", "recover":
	default:
		return domain.E(domain.Invalid, "unknown mail subcommand")
	}
	if req.Action == "show" || req.Action == "export" {
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			return domain.E(domain.Invalid, "mail %s requires an exact message ID before flags", req.Action)
		}
		req.ID = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("mail", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonMode := false
	fs.BoolVar(&jsonMode, "json", false, "structured output")
	mutation := req.Action == "serve" || req.Action == "export" || req.Action == "prune" || req.Action == "recover"
	if mutation {
		fs.BoolVar(&req.DryRun, "dry-run", false, "preview only")
		fs.StringVar(&req.Approval, "approve", "", "exact reviewed plan ID")
	}
	if req.Action == "serve" {
		fs.StringVar(&req.Executable, "mailpit", "/usr/local/bin/mailpit", "existing reviewed Mailpit v1.33.0 executable")
		fs.IntVar(&req.Policy.SMTPPort, "smtp-port", 1025, "loopback SMTP port")
		fs.IntVar(&req.Policy.APIPort, "api-port", 8026, "private loopback API port")
		fs.IntVar(&req.Policy.MaxMessages, "max-messages", 500, "finite retained message target")
		fs.IntVar(&req.Policy.MaxDays, "max-days", 7, "finite retained age in days")
	}
	if req.Action == "list" {
		fs.IntVar(&req.Offset, "offset", 0, "bounded listing offset")
	}
	if req.Action == "export" {
		fs.StringVar(&req.Attachment, "attachment", "", "attachment ID from mail show")
		fs.StringVar(&req.Output, "output", "", "new absolute export path")
	}
	if e := fs.Parse(args); e != nil {
		if e == flag.ErrHelp {
			_, e = io.WriteString(c.Out, mailHelp)
			return e
		}
		return domain.E(domain.Invalid, "%v", e)
	}
	if fs.NArg() != 0 {
		return domain.E(domain.Invalid, "unexpected mail arguments")
	}
	if c.Paths.Config == "" {
		var e error
		c.Paths, e = platform.UserPaths()
		if e != nil {
			return e
		}
	}
	output := func(v any) error {
		if jsonMode {
			return json.NewEncoder(c.Out).Encode(v)
		}
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return e
		}
		_, e = fmt.Fprintln(c.Out, string(b))
		return e
	}
	service := app.MailService{Store: state.Store{Paths: c.Paths}, Ready: func(r app.MailRecord) error {
		if jsonMode {
			return output(map[string]any{"event": "mail_ready", "smtp_port": r.Policy.SMTPPort, "api_port": r.Policy.APIPort})
		}
		_, e := fmt.Fprintf(c.Out, "Mail capture ready on 127.0.0.1:%d. Press Ctrl-C to stop.\n", r.Policy.SMTPPort)
		return e
	}}
	var value any
	var e error
	if mutation {
		if req.DryRun && req.Approval != "" {
			return domain.E(domain.Invalid, "dry-run cannot be combined with approval")
		}
		if req.Approval == "" {
			value, e = service.Plan(ctx, req)
		} else {
			value, e = service.Apply(ctx, req)
		}
	} else {
		value, e = service.Read(ctx, req)
	}
	if e != nil {
		return e
	}
	return output(value)
}
