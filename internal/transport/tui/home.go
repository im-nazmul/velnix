package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type commandItem struct {
	name, description string
	action            int
}

func (m *model) commands() []commandItem {
	items := []commandItem{{"Go home", "Your workspace at a glance", 0}, {"Browse projects", "Explore your linked projects", 1}, {"Explore runtimes", "See available language runtimes", 2}, {"View services", "Check your local web services", 3}, {"Check environment", "Find configuration issues", 4}, {"Refresh", "Read the current environment again", 5}, {"Keyboard shortcuts", "Learn your way around", 6}}
	out := []commandItem{}
	for _, item := range items {
		matches := true
		for _, word := range strings.Fields(strings.ToLower(m.command)) {
			if !strings.Contains(strings.ToLower(item.name+" "+item.description), word) {
				matches = false
				break
			}
		}
		if matches {
			out = append(out, item)
		}
	}
	return out
}
func (m *model) action(action int) tea.Cmd {
	if action == 5 {
		return m.load()
	}
	if action == 6 {
		m.home = false
		m.help = true
		m.helpOffset = 0
		return nil
	}
	m.home = action == 0
	m.screen = action
	m.detail = false
	m.technical = false
	m.help = false
	m.filter = ""
	m.entries = nil
	m.restoreName = ""
	m.cursor = 0
	return m.load()
}
func (m *model) friendlyView() string {
	w, h := max(1, m.width), max(1, m.height)
	if w < 20 || h < 7 {
		return fit("Velnix · ctrl+p commands", w)
	}
	contentWidth := min(68, w-6)
	lines := []string{}
	footer := "ctrl+p commands     1 projects     2 runtimes     q quit"
	if m.palette {
		lines = []string{accent.Render("What would you like to do?"), "", "› " + single(m.command) + "▏", muted.Render(strings.Repeat("─", contentWidth)), ""}
		items := m.commands()
		if len(items) == 0 {
			lines = append(lines, "No matching actions. Try “projects” or “services”.")
		}
		compact := h < 14
		if compact {
			lines = []string{accent.Render("Actions"), "› " + single(m.command) + "▏"}
		}
		rowHeight := 1
		if h >= 22 {
			rowHeight = 2
		}
		visibleRows := max(1, (h-4-len(lines))/rowHeight)
		start := max(0, m.commandCursor-visibleRows+1)
		for i := start; i < len(items); i++ {
			item := items[i]
			prefix := "  "
			if i == m.commandCursor {
				prefix = "› "
			}
			name := prefix + item.name
			if i == m.commandCursor {
				name = accent.Render(name)
			}
			lines = append(lines, name)
			if h >= 22 {
				lines = append(lines, muted.Render("  "+item.description))
			}
		}
		footer = "↑↓ choose     enter open     esc close"
	} else {
		brand := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("v e l n i x")
		lines = []string{brand, "", "A little space for your local projects.", muted.Render("Your projects, runtimes and services. All in one place."), ""}
		if m.loading {
			lines = append(lines, muted.Render("Taking a look at your environment…"))
		} else {
			lines = append(lines, m.homeSummary()...)
		}
		lines = append(lines, "", accent.Render("›  What would you like to explore?"), muted.Render("   Press enter to find an action"), "", "1  Projects         2  Runtimes", "3  Services         4  Environment", "", muted.Render("Just looking around. Nothing is started or changed."))
	}
	if !m.palette && h < 20 {
		lines = []string{accent.Render("velnix"), "Your local workspace.", "", accent.Render("› Enter to explore"), "1 Projects  ·  2 Runtimes", "3 Services  ·  4 Environment"}
	}
	capacity := max(1, h-4)
	if len(lines) > capacity {
		lines = lines[:capacity]
	}
	left := max(2, (w-contentWidth)/2)
	top := max(1, (h-len(lines)-3)/3)
	out := make([]string, top)
	for _, line := range lines {
		out = append(out, strings.Repeat(" ", left)+fit(line, contentWidth))
	}
	for len(out) < h-1 {
		out = append(out, "")
	}
	if w < 65 {
		if m.palette {
			footer = "↑↓ choose · enter open · esc close"
		} else {
			footer = "ctrl+p commands · enter explore · q quit"
		}
	}
	out = append(out, muted.Render("  "+fit(footer, w-3)))
	if len(out) > h {
		out = out[:h]
	}
	for i := range out {
		out[i] = ansi.Truncate(out[i], w, "")
	}
	return strings.Join(out, "\n")
}

// Home uses existing query evidence only; a saved certificate does not prove
// that HTTPS is running or trusted by the browser.
func (m *model) homeSummary() []string {
	projects, web, tls := "—", "—", "—"
	attention := 0
	empty := false
	for _, e := range m.entries {
		switch e.Name {
		case "Registered projects":
			var count int
			if json.Unmarshal([]byte(e.Detail), &count) == nil {
				projects = fmt.Sprint(count)
				empty = count == 0
			}
		case "Projects":
			projects = "unavailable"
		case "HTTP":
			web = summary(e)
		case "TLS":
			if strings.HasPrefix(e.Detail, "Unavailable:") {
				tls = "unavailable"
			} else if strings.TrimSpace(e.Detail) == "null" {
				tls = "not set up"
			} else {
				tls = "certificate saved"
			}
		}
		lower := strings.ToLower(e.Detail)
		if strings.HasPrefix(lower, "unavailable:") || strings.HasPrefix(lower, "error:") || strings.HasPrefix(lower, "warning:") {
			attention++
		}
	}
	out := []string{muted.Render("Projects  ") + projects + muted.Render("    Web  ") + web, muted.Render("HTTPS  ") + tls}
	if attention > 0 {
		out = append(out, lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("! Environment needs a look · press 4"))
	} else if empty {
		out = append(out, muted.Render("No projects yet? Open Projects to find your next step."))
	}
	return out
}
