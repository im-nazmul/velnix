package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"velnix/internal/app"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

func label(s string) string  { return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "php", "PHP") }
func single(s string) string { return strings.ReplaceAll(clean(s), "\n", " ") }
func summary(e app.TerminalEntry) string {
	var v any
	if json.Unmarshal([]byte(e.Detail), &v) != nil {
		line := single(strings.Split(e.Detail, "\n")[0])
		if strings.HasPrefix(line, "Unavailable:") {
			return "Unavailable"
		}
		if strings.HasPrefix(line, "Unsupported") {
			return "Unsupported"
		}
		return line
	}
	switch x := v.(type) {
	case nil:
		return "Not configured"
	case float64:
		return fmt.Sprintf("%g registered", x)
	case map[string]any:
		if _, ok := x["record"]; ok {
			if x["record"] == nil {
				return "Not configured"
			}
			if x["recovery_required"] == true {
				return "Recovery required"
			}
			if x["alive"] == true {
				if x["pending_routing"] == true {
					return "Running · routing pending"
				}
				return "Running"
			}
			return "Stopped"
		}
		if c, ok := x["candidates"].([]any); ok {
			return fmt.Sprintf("%d found · not probed", len(c))
		}
		if domain, ok := x["domain"].(string); ok {
			return domain
		}
		if os, ok := x["os"].(string); ok {
			return os
		}
	case []any:
		return fmt.Sprintf("%d available", len(x))
	}
	return "View details"
}

// Technical identifiers remain available explicitly, outside the everyday view.
func technicalKey(k string) bool {
	for _, part := range []string{"digest", "inode", "device", "fingerprint", "environment", "argv", "schema", "process", "pools"} {
		if strings.Contains(k, part) {
			return true
		}
	}
	return false
}
func readable(v any, depth int) []string {
	if depth > 6 {
		return []string{"Additional nested data available in technical view."}
	}
	switch x := v.(type) {
	case nil:
		return []string{"Not configured"}
	case map[string]any:
		if record, ok := x["record"]; ok {
			if record == nil {
				return []string{"Status: Not configured", "", "No owned workload has been configured."}
			}
			status := "Stopped"
			if x["alive"] == true {
				status = "Running"
			}
			if x["recovery_required"] == true {
				status = "Recovery required"
			}
			out := []string{"Status: " + status}
			if x["pending_routing"] == true {
				out = append(out, "Routing: Changes pending")
			}
			if reason, ok := x["routing_error"].(string); ok && reason != "" {
				out = append(out, "Routing: "+clean(reason))
			}
			return append(out, readable(record, depth+1)...)
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			if !technicalKey(k) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		out := []string{}
		for _, k := range keys {
			switch val := x[k].(type) {
			case map[string]any, []any:
				out = append(out, "", strings.ToUpper(label(k)))
				for _, line := range readable(val, depth+1) {
					out = append(out, "  "+line)
				}
			default:
				out = append(out, fmt.Sprintf("%s: %s", label(k), strings.Join(readable(val, depth+1), " ")))
			}
		}
		if len(out) == 0 {
			return []string{"No additional information"}
		}
		return out
	case []any:
		if len(x) == 0 {
			return []string{"None"}
		}
		out := []string{}
		for i, item := range x {
			if i >= 50 {
				out = append(out, "Further entries available in technical view.")
				break
			}
			if i > 0 {
				out = append(out, "")
			}
			out = append(out, readable(item, depth+1)...)
		}
		return out
	case bool:
		if x {
			return []string{"Yes"}
		}
		return []string{"No"}
	case string:
		return strings.Split(clean(x), "\n")
	default:
		return []string{fmt.Sprint(x)}
	}
}
func details(e app.TerminalEntry, technical bool) string {
	if technical {
		return clean(e.Detail)
	}
	// Project detail includes separately labelled metadata and web profile JSON.
	sections := strings.Split(e.Detail, "\n\n")
	out := []string{}
	for _, section := range sections {
		var v any
		source := section
		heading := ""
		if i := strings.IndexByte(section, '\n'); i >= 0 && !strings.HasPrefix(section, "{") && !strings.HasPrefix(section, "[") {
			heading = section[:i]
			source = section[i+1:]
		}
		if json.Unmarshal([]byte(source), &v) == nil {
			if heading != "" {
				out = append(out, "", strings.ToUpper(heading))
			}
			out = append(out, readable(v, 0)...)
		} else {
			out = append(out, clean(section))
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
func displayName(e app.TerminalEntry) string {
	return strings.TrimSuffix(single(e.Name), " candidates (not probed)")
}
func statusText(e app.TerminalEntry) string {
	text := summary(e)
	style := muted
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "unavailable"), strings.Contains(lower, "recovery required"), strings.HasPrefix(lower, "error:"):
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		text = "! " + text
	case strings.HasPrefix(lower, "warning:"), strings.Contains(lower, "pending"):
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		text = "~ " + text
	case text == "Running", strings.HasPrefix(lower, "ok:"):
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		text = "✓ " + text
	}
	return style.Render(text)
}
func (m *model) detailLayout() (int, int) {
	width := max(1, m.width)
	if width >= 30 {
		width -= 2
	}
	headers := 2
	if m.height >= 10 {
		headers++
	}
	if m.filter != "" || m.filtering {
		headers++
	}
	return width, max(1, m.height-headers-2)
}
func (m *model) detailBody(width, available int) []string {
	entries := m.visible()
	if len(entries) == 0 {
		return nil
	}
	e := entries[min(m.cursor, len(entries)-1)]
	heading := accent.Render(single(e.Name))
	if m.technical {
		heading += " · technical"
	}
	prefix := []string{heading}
	if available >= 5 {
		prefix = append(prefix, "")
	}
	return append(prefix, strings.Split(ansi.Hardwrap(details(e, m.technical), width, true), "\n")...)
}
func (m *model) maxDetailOffset() int {
	width, available := m.detailLayout()
	return max(0, len(m.detailBody(width, available))-available)
}
func fit(s string, w int) string { return ansi.Truncate(s, max(1, w), "…") }
func (m *model) View() string {
	if m.home || m.palette {
		return m.friendlyView()
	}
	w, h := max(1, m.width), max(1, m.height)
	if h < 5 {
		return fit("Velnix · "+screens[m.screen], w)
	}
	pad := 1
	if w < 30 {
		pad = 0
	}
	inner := max(1, w-pad*2)
	title := accent.Render("VELNIX") + "  " + muted.Render("workspace   ·   ctrl+p commands")
	if w < 50 {
		title = accent.Render("VELNIX") + "  " + screens[m.screen]
	}
	tabs := []string{}
	for i, s := range screens {
		if i == m.screen {
			tabs = append(tabs, accent.Render("["+s+"]"))
		} else {
			tabs = append(tabs, muted.Render(s))
		}
	}
	nav := strings.Join(tabs, "   ")
	if ansi.StringWidth(nav) > inner {
		nav = accent.Render(screens[m.screen]) + muted.Render("  · tab to switch")
	}
	head := []string{title, nav}
	if h >= 10 {
		head = append(head, "")
	}
	if m.filter != "" || m.filtering {
		head = append(head, "Filter: "+single(m.filter))
	}
	available := max(1, h-len(head)-2)
	entries := m.visible()
	body := []string{}
	footer := "↑↓ select   enter open   / filter   ? help   q quit"
	switch {
	case m.help:
		body = strings.Split(keyboardHelp, "\n")
		offset := min(m.helpOffset, max(0, len(body)-available))
		body = body[offset:]
		footer = "↑↓ scroll   ? close help   q quit"
	case m.loading:
		body = []string{"", "Loading " + strings.ToLower(screens[m.screen]) + "…"}
	case len(entries) == 0:
		if m.filter != "" {
			body = []string{"No matches for “" + single(m.filter) + "”.", "", "Esc clears the filter · / edits it"}
		} else {
			body = []string{"Nothing to display yet.", "Press r to refresh."}
		}
	case m.detail:
		body = m.detailBody(inner, available)
		offset := min(m.offset, max(0, len(body)-available))
		body = body[offset:]
		footer = fmt.Sprintf("%d–%d/%d   esc back   t technical   ? help", offset+1, min(offset+available, len(body)+offset), len(body)+offset)
	default:
		wide := inner >= 90 && available >= 10
		listWidth := inner
		if wide {
			listWidth = max(28, inner*2/5)
		}
		start := max(0, m.cursor-available+1)
		for i := start; i < len(entries) && len(body) < available; i++ {
			e := entries[i]
			mark := "  "
			if i == m.cursor {
				mark = "> "
			}
			nameWidth := max(8, listWidth*3/5-2)
			row := mark + displayName(e)
			if listWidth >= 36 {
				left := fit(row, nameWidth)
				row = left + strings.Repeat(" ", max(1, nameWidth-ansi.StringWidth(left)+2)) + fit(statusText(e), listWidth-nameWidth-3)
			}
			row = fit(row, listWidth)
			if i == m.cursor {
				row = accent.Render(row)
			}
			body = append(body, row)
		}
		if wide && len(entries) > 0 {
			e := entries[min(m.cursor, len(entries)-1)]
			previewWidth := inner - listWidth - 3
			preview := append([]string{accent.Render(single(e.Name)), fit(statusText(e), previewWidth), ""}, strings.Split(ansi.Hardwrap(details(e, false), previewWidth, true), "\n")...)
			for len(body) < available {
				body = append(body, "")
			}
			for i := range body {
				right := ""
				if i < len(preview) {
					right = fit(preview[i], previewWidth)
				}
				body[i] = body[i] + strings.Repeat(" ", max(0, listWidth-ansi.StringWidth(body[i]))) + muted.Render(" │ ") + right
			}
		}
		footer = fmt.Sprintf("%d/%d  enter open   r refresh   ? help   q quit", min(m.cursor+1, len(entries)), len(entries))
	}
	if len(body) > available {
		body = body[:available]
	}
	for len(body) < available {
		body = append(body, "")
	}
	lines := append(head, body...)
	if inner < 58 {
		footer = "↑↓ move  enter open  ? help  q quit"
		if m.detail {
			footer = "↑↓ scroll  esc back  t data"
		}
		if m.filtering {
			footer = "type to filter  enter done"
		}
		if m.help {
			footer = "↑↓ scroll  ? close"
		}
	}
	lines = append(lines, "", muted.Render(footer))
	if len(lines) > h {
		lines = lines[:h]
	}
	for i := range lines {
		lines[i] = strings.Repeat(" ", pad) + fit(lines[i], inner)
	}
	return strings.Join(lines, "\n")
}

const keyboardHelp = "KEYBOARD\n\nCtrl+P       Find an action\nTab / ← →    Switch screen\n↑ ↓ / j k    Select or scroll\nPageUp/Down  Page content\nHome/End     Beginning / end\nEnter        Open details\nEsc          Back / clear filter / home\n/            Filter by name\nr            Refresh\nt            Technical details\nq / Ctrl-C   Quit\n\nRead only. Nothing is started or changed."

func (m *model) maxHelpOffset() int {
	_, available := m.detailLayout()
	return max(0, len(strings.Split(keyboardHelp, "\n"))-available)
}
