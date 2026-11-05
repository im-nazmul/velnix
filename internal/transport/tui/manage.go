package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"velnix/internal/app"
	"velnix/internal/domain"
)

type Manager interface {
	Plan(context.Context, app.TerminalChange) (app.TerminalPlan, error)
	Apply(context.Context, app.TerminalPlan) error
}
type inputField struct{ label, value string }
type changeForm struct {
	operation, title string
	fields           []inputField
	focus            int
	stage            string
	plan             app.TerminalPlan
	offset           int
	confirm          string
	reviewed         bool
	message          string
}
type changeResult struct {
	token   int
	plan    app.TerminalPlan
	applied bool
	err     error
}

func (m *model) beginChange(action int) tea.Cmd {
	if _, ok := m.reader.(Manager); !ok {
		return nil
	}
	f := &changeForm{stage: "edit"}
	name := ""
	entries := m.visible()
	if m.screen == 1 && len(entries) > 0 {
		candidate := entries[min(m.cursor, len(entries)-1)].Name
		if domain.ValidProjectName(candidate) {
			name = candidate
		}
	}
	field := func(label, value string) { f.fields = append(f.fields, inputField{label, value}) }
	switch action {
	case 20:
		f.operation = "initialize"
		f.title = "Initialize private state"
	case 21:
		f.operation = "link"
		f.title = "Link a project"
		field("Project name", "")
		field("Absolute project path", "")
		field("Document root (relative or absolute)", ".")
	case 22:
		f.operation = "unlink"
		f.title = "Unlink a project"
		field("Project name", name)
	case 23:
		f.operation = "select-runtime"
		f.title = "Select an existing runtime"
		field("Project name", name)
		field("Kind: php, node, bun or python", "php")
		field("Absolute executable path", "")
		field("Exact version", "")
	case 24:
		f.operation = "clear-runtime"
		f.title = "Clear a runtime selection"
		field("Project name", name)
		field("Kind: php, node, bun or python", "php")
	case 25:
		f.operation = "start-http"
		f.title = "Start web serving"
		field("Installed Nginx executable", "/usr/sbin/nginx")
		field("Loopback HTTP port", "8080")
		field("Use existing HTTPS certificate: yes or no", "no")
		field("Loopback HTTPS port", "8443")
	case 26:
		f.operation = "stop-http"
		f.title = "Stop owned web serving"
	case 27:
		f.operation = "reload-http"
		f.title = "Reload owned web routes"
	case 28:
		f.operation = "start-fpm"
		f.title = "Start a project PHP pool"
		field("Project name", name)
		field("Absolute PHP-FPM executable", "")
		field("Exact version", "")
		field("Web profile: php or front-controller", "php")
		field("Upload directories (comma separated)", "uploads")
	case 29:
		f.operation = "stop-fpm"
		f.title = "Stop a project PHP pool"
		field("Project name", name)
	default:
		return nil
	}
	m.form = f
	m.changeToken++
	if len(f.fields) == 0 {
		return m.previewChange()
	}
	return nil
}
func (f *changeForm) change() (app.TerminalChange, error) {
	c := app.TerminalChange{Operation: f.operation}
	v := func(i int) string { return f.fields[i].value }
	switch f.operation {
	case "link":
		c.Name = v(0)
		c.Root = v(1)
		c.DocumentRoot = v(2)
	case "unlink", "stop-fpm":
		c.Name = v(0)
	case "select-runtime":
		c.Name = v(0)
		c.Kind = v(1)
		c.Executable = v(2)
		c.Version = v(3)
	case "clear-runtime":
		c.Name = v(0)
		c.Kind = v(1)
	case "start-http":
		c.Executable = v(0)
		port, e := strconv.Atoi(v(1))
		if e != nil || port < 1024 || port > 65535 {
			return c, fmt.Errorf("choose a loopback port between 1024 and 65535")
		}
		c.Port = port
		if v(2) != "yes" && v(2) != "no" {
			return c, fmt.Errorf("choose yes or no for HTTPS")
		}
		c.HTTPS = v(2) == "yes"
		httpsPort, e := strconv.Atoi(v(3))
		if e != nil || httpsPort < 1024 || httpsPort > 65535 {
			return c, fmt.Errorf("choose an HTTPS port between 1024 and 65535")
		}
		c.HTTPSPort = httpsPort
	case "start-fpm":
		c.Name = v(0)
		c.Executable = v(1)
		c.Version = v(2)
		c.Profile = v(3)
		c.Uploads = v(4)
	}
	if strings.Contains(f.operation, "runtime") && c.Kind != "php" && c.Kind != "node" && c.Kind != "bun" && c.Kind != "python" {
		return c, fmt.Errorf("choose php, node, bun or python")
	}
	return c, nil
}
func (m *model) previewChange() tea.Cmd {
	c, e := m.form.change()
	if e != nil {
		m.form.message = e.Error()
		return nil
	}
	manager := m.reader.(Manager)
	m.form.stage = "planning"
	m.form.message = ""
	m.changeToken++
	token := m.changeToken
	ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
	m.changeCancel = cancel
	return func() tea.Msg {
		defer cancel()
		p, e := manager.Plan(ctx, c)
		return changeResult{token: token, plan: p, err: e}
	}
}
func (m *model) updateChange(key tea.KeyMsg) tea.Cmd {
	f := m.form
	k := key.String()
	if f.stage == "applying" {
		return nil
	}
	if k == "esc" {
		if m.changeCancel != nil {
			m.changeCancel()
		}
		m.changeToken++
		m.form = nil
		return m.load()
	}
	if m.width < 40 || m.height < 12 {
		return nil
	}
	if f.stage == "planning" {
		return nil
	}
	if f.stage == "error" {
		limit := max(0, len(m.changeErrorLines())-max(1, m.height-5))
		switch k {
		case "down":
			f.offset = min(limit, f.offset+1)
		case "up":
			f.offset = max(0, f.offset-1)
		case "pgdown":
			f.offset = min(limit, f.offset+max(1, m.height-5))
		case "pgup":
			f.offset = max(0, f.offset-max(1, m.height-5))
		case "home":
			f.offset = 0
		case "end":
			f.offset = limit
		}
	}
	if f.stage == "done" || f.stage == "error" {
		if k == "enter" {
			m.form = nil
			return m.load()
		}
		return nil
	}
	if f.stage == "review" {
		switch k {
		case "down":
			f.offset++
		case "up":
			f.offset = max(0, f.offset-1)
		case "pgdown":
			f.offset += max(1, m.height-7)
		case "pgup":
			f.offset = max(0, f.offset-max(1, m.height-7))
		case "home":
			f.offset = 0
		case "end":
			f.offset = m.reviewLimit()
		case "backspace":
			r := []rune(f.confirm)
			if len(r) > 0 {
				f.confirm = string(r[:len(r)-1])
			}
		case "enter":
			if f.reviewed && f.confirm == "APPLY" {
				f.stage = "applying"
				token := m.changeToken
				p := f.plan
				manager := m.reader.(Manager)
				ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
				m.changeCancel = cancel
				return func() tea.Msg {
					defer cancel()
					e := manager.Apply(ctx, p)
					return changeResult{token: token, applied: true, err: e}
				}
			}
		default:
			if (key.Type == tea.KeyRunes || key.String() == " ") && len(f.confirm) < 5 && f.reviewed {
				addition := strings.ToUpper(inputText(key))
				if len(f.confirm)+len(addition) <= 5 {
					f.confirm += addition
				}
			}
		}
		f.offset = min(f.offset, m.reviewLimit())
		if f.offset == m.reviewLimit() {
			f.reviewed = true
		}
		return nil
	}
	switch k {
	case "ctrl+u":
		f.fields[f.focus].value = ""
	case "tab", "enter":
		if f.focus == len(f.fields)-1 {
			return m.previewChange()
		}
		f.focus++
	case "shift+tab", "up":
		f.focus = max(0, f.focus-1)
	case "down":
		f.focus = min(len(f.fields)-1, f.focus+1)
	case "backspace":
		r := []rune(f.fields[f.focus].value)
		if len(r) > 0 {
			f.fields[f.focus].value = string(r[:len(r)-1])
		}
	default:
		if (key.Type == tea.KeyRunes || key.String() == " ") && len(f.fields[f.focus].value) < 4096 {
			f.fields[f.focus].value = appendInput(f.fields[f.focus].value, key, 4096)
		}
	}
	return nil
}
func (m *model) reviewLines() []string {
	return strings.Split(ansi.Hardwrap(clean(m.form.plan.Detail), max(1, m.width-4), true), "\n")
}
func (m *model) reviewLimit() int { return max(0, len(m.reviewLines())-max(1, m.height-7)) }
func (m *model) changeView() string {
	f := m.form
	w, h := max(1, m.width), max(1, m.height)
	if w < 40 || h < 12 {
		message := "Resize to at least 40×12 to manage changes. Esc cancels."
		if f.stage == "applying" {
			message = "Applying… Ctrl-C cancels the session."
		}
		return fit(message, w)
	}
	lines := []string{accent.Render(f.title), ""}
	footer := "tab next · shift-tab back · ctrl+u clear · esc cancel"
	switch f.stage {
	case "edit":
		// Display the focused field even in a short terminal.
		start := max(0, f.focus-max(1, (h-6)/3)+1)
		for i := start; i < len(f.fields); i++ {
			prefix := "  "
			if i == f.focus {
				prefix = "› "
			}
			lines = append(lines, prefix+f.fields[i].label, "  "+single(f.fields[i].value)+map[bool]string{true: "▏", false: ""}[i == f.focus], "")
		}
		if f.message != "" {
			lines = append([]string{accent.Render(f.title), single(f.message)}, lines[2:]...)
		}
	case "planning":
		lines = append(lines, "Preparing a preview. No changes applied…")
		footer = "esc cancel"
	case "review":
		body := m.reviewLines()
		offset := min(f.offset, m.reviewLimit())
		available := max(1, h-7)
		end := min(len(body), offset+available)
		lines = append(lines, body[offset:end]...)
		footer = fmt.Sprintf("%d–%d/%d · arrows/page/end scroll · esc cancel", offset+1, end, len(body))
		prompt := "Read to the end before approval (End jumps there)."
		if f.reviewed {
			prompt = "Type APPLY, then Enter: " + f.confirm
		}
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		lines = append(lines, accent.Render(prompt))
	case "applying":
		lines = append(lines, "Applying the exact reviewed plan…", "", "Please wait. Ctrl-C exits and cancels the session.")
		footer = "Changes may need recovery if interrupted; inspect Environment."
	case "done":
		lines = append(lines, "Change applied.", "", "Press Enter to refresh your workspace.")
		footer = "enter continue · esc back"
	case "error":
		body := m.changeErrorLines()
		available := max(1, h-5)
		offset := min(f.offset, max(0, len(body)-available))
		end := min(len(body), offset+available)
		lines = append(lines, body[offset:end]...)
		footer = "↑↓ scroll · enter continue · esc back"
	}
	limit := max(1, h-2)
	if len(lines) > limit {
		lines = lines[:limit]
	}
	for len(lines) < h-1 {
		lines = append(lines, "")
	}
	lines = append(lines, muted.Render(footer))
	for i := range lines {
		lines[i] = fit(lines[i], w)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func (m *model) changeErrorLines() []string {
	return strings.Split(ansi.Hardwrap("Change could not complete.\n\n"+clean(m.form.message)+"\n\nNo automatic retry. Inspect state/services before retrying.", max(1, m.width-4), true), "\n")
}
