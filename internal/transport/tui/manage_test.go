package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"velnix/internal/app"
)

type fakeManager struct {
	fakeReader
	applied int
}

func (f *fakeManager) Plan(context.Context, app.TerminalChange) (app.TerminalPlan, error) {
	return app.TerminalPlan{ID: "exact-plan", Detail: strings.Repeat("Reviewed effect\n", 35)}, nil
}
func (f *fakeManager) Apply(context.Context, app.TerminalPlan) error { f.applied++; return nil }
func TestManagementRequiresFullReviewAndExplicitApproval(t *testing.T) {
	manager := &fakeManager{}
	m := &model{ctx: context.Background(), reader: manager, width: 80, height: 15}
	cmd := m.beginChange(20)
	if manager.applied != 0 || m.form.stage != "planning" {
		t.Fatal("form applied eagerly")
	}
	m.Update(cmd())
	key(m, "APPLY")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if manager.applied != 0 || m.form.confirm != "" {
		t.Fatal("accepted before review")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !m.form.reviewed {
		t.Fatal("end did not expose approval")
	}
	key(m, "APPLY")
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.form.stage != "applying" {
		t.Fatal("approval not submitted")
	}
	m.Update(cmd())
	if manager.applied != 1 || m.form.stage != "done" {
		t.Fatal("exact approval did not apply once")
	}
}
func TestManagementCancelDiscardsLatePlanAndKeepsBoundedView(t *testing.T) {
	manager := &fakeManager{}
	m := &model{ctx: context.Background(), reader: manager, width: 40, height: 8}
	cmd := m.beginChange(20)
	oldToken := m.changeToken
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(cmd())
	if m.form != nil || m.changeToken == oldToken || manager.applied != 0 {
		t.Fatal("canceled plan leaked")
	}
	m.beginChange(21)
	m.form.fields[0].value = "one"
	m.form.focus = 2
	for _, size := range [][2]int{{40, 8}, {10, 5}, {1, 1}} {
		m.width = size[0]
		m.height = size[1]
		view := m.View()
		if len(strings.Split(view, "\n")) > size[1] {
			t.Fatal("height overflow")
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow")
			}
		}
	}
	m.form.stage = "review"
	m.form.plan = app.TerminalPlan{ID: "old", Detail: "effect"}
	m.changeToken++
	m.Update(changeResult{token: m.changeToken, err: context.Canceled})
	if m.form.stage != "error" || manager.applied != 0 {
		t.Fatal("failed approval retried")
	}
}

func TestManagementSpacesAndActionRanking(t *testing.T) {
	m := &model{ctx: context.Background(), reader: &fakeManager{}, width: 80, height: 24}
	m.command = "Link project"
	items := m.commands()
	if len(items) == 0 || items[0].action != 21 {
		t.Fatal("exact action not first", items)
	}
	m.beginChange(21)
	m.form.focus = 1
	key(m, "/tmp/project")
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	key(m, "folder")
	if m.form.fields[1].value != "/tmp/project folder" {
		t.Fatal("space lost", m.form.fields[1].value)
	}
}

func TestPreviewErrorsKeepInputsAndInvalidateApproval(t *testing.T) {
	m := &model{ctx: context.Background(), reader: &fakeManager{}, width: 80, height: 24}
	m.beginChange(21)
	m.form.fields[0].value = "bad name"
	m.form.stage = "planning"
	m.Update(changeResult{token: m.changeToken, err: context.DeadlineExceeded})
	if m.form.stage != "edit" || m.form.fields[0].value != "bad name" || m.form.plan.ID != "" {
		t.Fatal("preview failure lost inputs or retained approval")
	}
	m.form.focus = 1
	key(m, strings.Repeat("x", 10000))
	if len([]rune(m.form.fields[1].value)) > 4096 {
		t.Fatal("unbounded paste")
	}
}

func TestSmallManagementCannotApprove(t *testing.T) {
	manager := &fakeManager{}
	m := &model{ctx: context.Background(), reader: manager, width: 80, height: 24}
	cmd := m.beginChange(20)
	m.Update(cmd())
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
	key(m, "APPLY")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if manager.applied != 0 || m.form.stage != "review" || !strings.Contains(m.View(), "Resize") {
		t.Fatal("small terminal accepted approval")
	}
}

func TestApplyErrorDetailsRemainReadableAndCannotRetry(t *testing.T) {
	manager := &fakeManager{}
	m := &model{ctx: context.Background(), reader: manager, width: 42, height: 12, form: &changeForm{stage: "applying", offset: 100}, changeToken: 2}
	m.Update(changeResult{token: 2, applied: true, err: fmt.Errorf("%s final failure detail", strings.Repeat("long failure description ", 40))})
	if m.form.stage != "error" || m.form.offset != 0 {
		t.Fatal("failed operation left review cursor")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(ansi.Strip(m.View()), "final failure detail") {
		t.Fatal("error detail unreachable", m.View())
	}
	key(m, "APPLY")
	if manager.applied != 0 {
		t.Fatal("error replayed approval")
	}
}

func TestPreviewErrorCannotInjectTerminalControls(t *testing.T) {
	m := &model{ctx: context.Background(), reader: &fakeManager{}, width: 80, height: 20}
	m.beginChange(21)
	m.form.stage = "planning"
	m.Update(changeResult{token: m.changeToken, err: fmt.Errorf("bad path\x1b[2J\x1b]52;c;payload\a\u202e")})
	view := m.View()
	if strings.Contains(view, "\x1b[2J") || strings.Contains(view, "payload") || strings.Contains(view, "\u202e") {
		t.Fatal("untrusted preview error reached terminal")
	}
}
