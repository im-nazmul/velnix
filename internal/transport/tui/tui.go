// Package tui presents application queries and reviewed operations in a terminal UI.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"velnix/internal/app"
)

type Reader interface {
	Read(context.Context, int) []app.TerminalEntry
}
type result struct {
	generation int
	entries    []app.TerminalEntry
}

var screens = []string{"Overview", "Projects", "Runtimes", "Services", "Diagnostics"}

type model struct {
	ctx                                               context.Context
	reader                                            Reader
	cancel                                            context.CancelFunc
	generation, screen, width, height, cursor, offset int
	entries                                           []app.TerminalEntry
	loading, detail, filtering, help                  bool
	filter                                            string
	technical                                         bool
	restoreName                                       string
	helpOffset                                        int
	home                                              bool
	palette                                           bool
	command                                           string
	commandCursor                                     int
	form                                              *changeForm
	changeToken                                       int
	changeCancel                                      context.CancelFunc
}

func (m *model) load() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	m.cancel = cancel
	m.generation++
	gen, screen := m.generation, m.screen
	if entries := m.visible(); len(entries) > 0 {
		m.restoreName = entries[min(m.cursor, len(entries)-1)].Name
	}
	m.loading = true
	m.entries = nil
	m.offset = 0
	return func() tea.Msg {
		defer cancel()
		entries := m.reader.Read(ctx, screen)
		if ctx.Err() != nil {
			entries = []app.TerminalEntry{{Name: "Read interrupted", Detail: ctx.Err().Error()}}
		}
		return result{gen, entries}
	}
}
func (m *model) Init() tea.Cmd { return m.load() }
func (m *model) visible() []app.TerminalEntry {
	out := []app.TerminalEntry{}
	for _, e := range m.entries {
		if m.filter == "" || strings.Contains(strings.ToLower(clean(e.Name)), strings.ToLower(m.filter)) {
			out = append(out, e)
		}
	}
	return out
}
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, min(v.Width, 500))
		m.height = max(1, min(v.Height, 200))
	case changeResult:
		if m.form == nil || v.token != m.changeToken {
			return m, nil
		}
		if v.err != nil {
			stage := "error"
			if m.form.stage == "planning" && len(m.form.fields) > 0 && !v.applied {
				stage = "edit"
			}
			m.form.stage = stage
			m.form.plan = app.TerminalPlan{}
			m.form.message = v.err.Error()
			m.form.offset = 0
		} else if v.applied {
			m.form.stage = "done"
		} else {
			m.form.plan = v.plan
			m.form.stage = "review"
			m.form.reviewed = m.reviewLimit() == 0
		}
		return m, nil
	case result:
		if v.generation == m.generation {
			m.entries = v.entries
			m.loading = false
			visible := m.visible()
			m.cursor = min(m.cursor, max(0, len(visible)-1))
			for i, e := range visible {
				if e.Name == m.restoreName {
					m.cursor = i
					break
				}
			}
			m.restoreName = ""
		}
	case tea.KeyMsg:
		key := v.String()
		if key == "ctrl+c" {
			if m.changeCancel != nil {
				m.changeCancel()
			}
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if m.form != nil {
			return m, m.updateChange(v)
		}
		if key == "ctrl+p" {
			m.palette = !m.palette
			m.command = ""
			m.commandCursor = 0
			return m, nil
		}
		if m.palette {
			switch key {
			case "esc":
				m.palette = false
			case "up":
				m.commandCursor = max(0, m.commandCursor-1)
			case "down":
				m.commandCursor = min(max(0, len(m.commands())-1), m.commandCursor+1)
			case "backspace":
				r := []rune(m.command)
				if len(r) > 0 {
					m.command = string(r[:len(r)-1])
				}
				m.commandCursor = 0
			case "enter":
				items := m.commands()
				if len(items) > 0 {
					action := items[min(m.commandCursor, len(items)-1)].action
					m.palette = false
					return m, m.action(action)
				}
			default:
				if (v.Type == tea.KeyRunes || v.String() == " ") && len(m.command) < 80 {
					m.command = appendInput(m.command, v, 80)
					m.commandCursor = 0
				}
			}
			return m, nil
		}
		if m.home {
			switch key {
			case "q":
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			case "enter":
				m.palette = true
				m.command = ""
				m.commandCursor = 0
			case "1", "2", "3", "4":
				return m, m.action(int(key[0] - '0'))
			case "r":
				return m, m.load()
			case "tab", "right", "l":
				return m, m.action(1)
			case "?":
				m.palette = true
				m.command = "Keyboard"
				m.commandCursor = 0
			}
			return m, nil
		}
		if m.help {
			switch key {
			case "esc", "backspace", "?":
				m.help = false
			case "down", "j":
				m.helpOffset = min(m.helpOffset+1, m.maxHelpOffset())
			case "up", "k":
				m.helpOffset = max(0, m.helpOffset-1)
			case "pgdown":
				m.helpOffset = min(m.maxHelpOffset(), m.helpOffset+max(1, m.height-5))
			case "pgup":
				m.helpOffset = max(0, m.helpOffset-max(1, m.height-5))
			case "q":
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit
			}
			return m, nil
		}
		if m.filtering {
			switch key {
			case "esc", "enter":
				m.filtering = false
			case "backspace":
				r := []rune(m.filter)
				if len(r) > 0 {
					m.filter = string(r[:len(r)-1])
				}
			default:
				if (v.Type == tea.KeyRunes || v.String() == " ") && len(m.filter) < 128 {
					m.filter = appendInput(m.filter, v, 128)
				}
			}
			m.cursor = 0
			m.offset = 0
			return m, nil
		}
		switch key {
		case "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "t":
			if !m.detail {
				return m, nil
			}
			m.technical = !m.technical
			m.offset = 0
		case "?":
			m.help = !m.help
			m.helpOffset = 0
		case "esc", "backspace":
			if !m.detail && m.filter == "" {
				return m, m.action(0)
			}
			wasDetail := m.detail
			m.detail = false
			m.technical = false
			m.help = false
			m.offset = 0
			if !wasDetail {
				m.filter = ""
				m.cursor = 0
			}
		case "/":
			m.filtering = true
			m.detail = false
			m.offset = 0
		case "r":
			return m, m.load()
		case "tab", "right", "l", "shift+tab", "left", "h":
			delta := 1
			if key == "left" || key == "h" || key == "shift+tab" {
				delta = -1
			}
			m.screen = (m.screen + delta + len(screens)) % len(screens)
			m.detail = false
			m.technical = false
			m.help = false
			m.entries = nil
			m.restoreName = ""
			m.cursor = 0
			m.filter = ""
			return m, m.load()
		case "home":
			if !m.detail {
				m.cursor = 0
			}
			m.offset = 0
		case "end":
			if m.detail {
				m.offset = m.maxDetailOffset()
			}
			if !m.detail {
				m.cursor = max(0, len(m.visible())-1)
			}
		case "enter":
			m.detail = true
			m.offset = 0
		case "down", "j":
			if m.detail {
				m.offset++
			} else {
				m.cursor = min(m.cursor+1, max(0, len(m.visible())-1))
			}
		case "up", "k":
			if m.detail {
				m.offset = max(0, m.offset-1)
			} else {
				m.cursor = max(0, m.cursor-1)
			}
		case "pgdown":
			if m.detail {
				m.offset += max(1, m.height-5)
			} else {
				m.cursor = min(m.cursor+max(1, m.height-5), max(0, len(m.visible())-1))
			}
		case "pgup":
			m.offset = max(0, m.offset-max(1, m.height-5))
			if !m.detail {
				m.cursor = max(0, m.cursor-max(1, m.height-5))
			}
		}
	}
	if m.detail {
		m.offset = min(m.offset, m.maxDetailOffset())
	}
	return m, nil
}

// Strip complete ANSI sequences, then remove remaining controls including C1 and
// bidi formatting. The byte cap bounds rendering independently of source size.
func clean(s string) string {
	if len(s) > 32768 {
		s = s[:32768] + "… [truncated]"
	}
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}
func Run(ctx context.Context, in io.Reader, out io.Writer, reader Reader) (resultErr error) {
	input, ok := in.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(input.Fd()) {
		return fmt.Errorf("tui requires interactive input and output terminals; use the CLI for scripts")
	}
	output, ok := out.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(output.Fd()) {
		return fmt.Errorf("tui requires interactive input and output terminals; use the CLI for scripts")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if session, ok := reader.(interface{ Close() error }); ok {
		defer func() { resultErr = errors.Join(resultErr, session.Close()) }()
	}
	m := &model{ctx: runCtx, reader: reader, width: 80, height: 24, home: true}
	defer func() {
		if m.cancel != nil {
			m.cancel()
		}
		if m.changeCancel != nil {
			m.changeCancel()
		}
	}()
	_, err := tea.NewProgram(m, tea.WithContext(runCtx), tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithoutSignalHandler()).Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, tea.ErrProgramKilled) {
		return context.Canceled
	}
	return err
}

func inputText(k tea.KeyMsg) string {
	if k.String() == " " {
		return " "
	}
	return single(string(k.Runes))
}

func appendInput(current string, k tea.KeyMsg, limit int) string {
	r := []rune(current + inputText(k))
	if len(r) > limit {
		r = r[:limit]
	}
	return string(r)
}
