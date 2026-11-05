package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"velnix/internal/app"
)

type fakeReader struct{ canceled bool }

func (f *fakeReader) Read(ctx context.Context, screen int) []app.TerminalEntry {
	f.canceled = ctx.Err() != nil
	return []app.TerminalEntry{{Name: screens[screen], Detail: "detail"}}
}
func key(m *model, k string) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return cmd
}
func TestNavigationCancelsAndDiscardsStaleReads(t *testing.T) {
	f := &fakeReader{}
	m := &model{ctx: context.Background(), reader: f, width: 80, height: 24}
	old := m.Init()
	next := key(m, "l")
	if m.screen != 1 {
		t.Fatal("navigation failed")
	}
	m.Update(old())
	if !f.canceled || !m.loading || len(m.entries) != 0 {
		t.Fatal("stale result applied or prior query not canceled")
	}
	m.Update(next())
	if m.loading || m.entries[0].Name != "Projects" {
		t.Fatal("current query not applied")
	}
	key(m, "q")
	if m.ctx.Err() != nil {
		t.Fatal("model canceled caller context")
	}
}
func TestFilteringDetailsAndSmallTerminal(t *testing.T) {
	m := &model{ctx: context.Background(), width: 24, height: 7, entries: []app.TerminalEntry{{Name: "Alpha", Detail: strings.Repeat("wide 世界 detail\n", 40)}, {Name: "Beta", Detail: "other"}}}
	key(m, "/")
	key(m, "B")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.visible()) != 1 || m.visible()[0].Name != "Beta" {
		t.Fatal("filter failed")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.detail || !strings.Contains(m.View(), "other") {
		t.Fatal("details unavailable")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.cursor = 0
	m.detail = true
	for _, size := range [][2]int{{24, 7}, {8, 3}, {1, 1}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.offset = 99999
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Fatal("height overflow")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow", line)
			}
		}
	}
}
func TestUntrustedTextAndNonTTY(t *testing.T) {
	raw := "safe\x1b[2J\x1b]52;c;secret\a\u202eevil\x9b\x00"
	got := clean(raw)
	if strings.ContainsAny(got, "\x1b\a\u202e\x9b\x00") || strings.Contains(got, "secret") {
		t.Fatalf("unsafe: %q", got)
	}
	if len(clean(strings.Repeat("a", 100000))) > 33000 {
		t.Fatal("unbounded display")
	}
	if err := Run(context.Background(), strings.NewReader(""), &strings.Builder{}, &fakeReader{}); err == nil {
		t.Fatal("accepted non-terminal")
	}
}

func TestReadableDetailsAndResponsivePreview(t *testing.T) {
	e := app.TerminalEntry{Name: "HTTP", Detail: `{"record":null,"alive":false,"registry_digest":"secret-id","pending_routing":false}`}
	m := &model{ctx: context.Background(), width: 110, height: 24, entries: []app.TerminalEntry{e}}
	view := ansi.Strip(m.View())
	t.Log("\n" + view)
	if !strings.Contains(view, "Not configured") || !strings.Contains(view, "│") || strings.Contains(view, "secret-id") {
		t.Fatal(view)
	}
	key(m, "enter")
	view = ansi.Strip(m.View())
	if strings.Contains(view, `"record"`) || strings.Contains(view, "secret-id") {
		t.Fatal("raw technical data leaked into normal detail", view)
	}
	key(m, "t")
	if !strings.Contains(m.View(), "secret-id") {
		t.Fatal("technical detail unavailable")
	}
	m.width = 45
	m.detail = false
	view = ansi.Strip(m.View())
	if strings.Contains(view, "│") {
		t.Fatal("narrow terminal retained split layout")
	}
}

func TestRefreshPreservesSelectionAndFilteredBack(t *testing.T) {
	m := &model{ctx: context.Background(), reader: &fakeReader{}, width: 90, height: 18, entries: []app.TerminalEntry{{Name: "Alpha"}, {Name: "Beta"}}, cursor: 1}
	m.load()
	m.Update(result{generation: m.generation, entries: []app.TerminalEntry{{Name: "Beta"}, {Name: "Alpha"}}})
	if m.cursor != 0 || m.visible()[m.cursor].Name != "Beta" {
		t.Fatal("refresh lost selected identity")
	}
	m.filter = "Beta"
	key(m, "enter")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.detail || m.filter != "Beta" {
		t.Fatal("back cleared filter")
	}
	key(m, "t")
	if m.technical {
		t.Fatal("list changed technical detail mode")
	}
}
func TestDetailPagingKeepsSelectionAndClampsScroll(t *testing.T) {
	m := &model{ctx: context.Background(), width: 60, height: 12, cursor: 1, detail: true, entries: []app.TerminalEntry{{Name: "Alpha"}, {Name: "Beta", Detail: strings.Repeat("a line\n", 35)}}}
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 1 {
		t.Fatal("paging changed selected row")
	}
	for i := 0; i < 100; i++ {
		key(m, "j")
	}
	if m.offset != m.maxDetailOffset() {
		t.Fatal("scroll offset exceeded content")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.offset != 0 || m.cursor != 1 {
		t.Fatal("home changed selected item")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.offset != m.maxDetailOffset() || m.cursor != 1 {
		t.Fatal("end did not reach content end")
	}
	before := m.offset
	key(m, "k")
	if m.offset != before-1 {
		t.Fatal("scroll did not move back immediately")
	}
	key(m, "?")
	for i := 0; i < 9; i++ {
		key(m, "j")
	}
	if m.cursor != 1 || m.offset != before-1 || !strings.Contains(ansi.Strip(m.View()), "Quit") {
		t.Fatal("help scrolling disturbed underlying detail")
	}
}
func TestStatusMeaningSurvivesWithoutColor(t *testing.T) {
	for _, tt := range []struct{ detail, want string }{{`{"record":{},"alive":true}`, "✓ Running"}, {"Unavailable: permission denied", "! Unavailable"}, {"warning: unsupported platform", "~ warning:"}, {`{"record":{},"recovery_required":true}`, "! Recovery required"}} {
		got := ansi.Strip(statusText(app.TerminalEntry{Detail: tt.detail}))
		if !strings.Contains(got, tt.want) {
			t.Fatal(got)
		}
	}
}

func TestFriendlyHomeAndActionPicker(t *testing.T) {
	m := &model{ctx: context.Background(), reader: &fakeReader{}, width: 100, height: 30, home: true}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "What would you like to explore?") || strings.Contains(view, "registry") {
		t.Fatal(view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	key(m, "projects")
	if len(m.commands()) != 1 {
		t.Fatal("action search failed")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.home || m.screen != 1 || m.palette {
		t.Fatal("project action did not navigate")
	}
	m.Update(cmd())
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != 1 || m.palette {
		t.Fatal("palette dismissal changed view")
	}
	for _, size := range [][2]int{{100, 30}, {40, 12}, {18, 6}, {1, 1}} {
		m.home = true
		m.width = size[0]
		m.height = size[1]
		for _, line := range strings.Split(m.View(), "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("home overflow")
			}
		}
		if len(strings.Split(m.View(), "\n")) > size[1] {
			t.Fatal("home height overflow")
		}
	}
}

func TestHomeUsesEvidenceWithoutOverstatingHTTPS(t *testing.T) {
	m := &model{home: true, width: 100, height: 28, entries: []app.TerminalEntry{{Name: "Registered projects", Detail: "0"}, {Name: "HTTP", Detail: `{"record":null,"alive":false}`}, {Name: "TLS", Detail: `{"phase":"issued"}`}}}
	view := ansi.Strip(m.View())
	for _, want := range []string{"Projects  0", "Not configured", "certificate saved", "No projects yet?"} {
		if !strings.Contains(view, want) {
			t.Fatal(view)
		}
	}
	if strings.Contains(view, "HTTPS  Running") || strings.Contains(view, "trusted") {
		t.Fatal("saved certificate overstated", view)
	}
	m.entries = []app.TerminalEntry{{Name: "Projects", Detail: "Unavailable: corrupt state"}, {Name: "state", Detail: "error: needs recovery"}}
	view = ansi.Strip(m.View())
	if !strings.Contains(view, "unavailable") || !strings.Contains(view, "press 4") || strings.Contains(view, "No projects yet?") {
		t.Fatal("failed read presented as empty", view)
	}
}
func TestActionSearchMatchesWordsInAnyOrder(t *testing.T) {
	m := &model{command: "projects browse"}
	if items := m.commands(); len(items) != 1 || items[0].action != 1 {
		t.Fatal(items)
	}
}

func TestSmallPickerKeepsChosenActionVisible(t *testing.T) {
	m := &model{width: 40, height: 8, palette: true, commandCursor: 6}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Keyboard shortcuts") {
		t.Fatal("selected action hidden", view)
	}
	m.palette = false
	m.help = true
	m.height = 7
	m.helpOffset = 0
	for i := 0; i < 100; i++ {
		key(m, "j")
	}
	if !strings.Contains(ansi.Strip(m.View()), "Nothing is started") {
		t.Fatal("help bottom inaccessible", m.View())
	}
}
