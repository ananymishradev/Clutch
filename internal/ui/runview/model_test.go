package runview

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelStageTransitionsAndResize(t *testing.T) {
	now := time.Now()
	m := New([]string{"manifest", "cache"}, 600, false, nil)
	m.Now = now
	m = update(m, StageMsg{Name: "manifest", Status: "started", Detail: "loading"})
	m.Now = now.Add(2 * time.Second)
	m = update(m, StageMsg{Name: "manifest", Status: "done", Detail: "case"})
	m = update(m, StageMsg{Name: "cache", Status: "started", Detail: strings.Repeat("detail", 30)})
	m = update(m, tea.WindowSizeMsg{Width: 50})
	view := m.View()
	if !strings.Contains(view, "manifest") || !strings.Contains(view, "…") {
		t.Fatalf("view did not render transitions/truncation: %q", view)
	}
}

func TestModelDetach(t *testing.T) {
	detached := false
	m := New([]string{"manifest"}, 0, false, func() { detached = true })
	m = update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.Detached || !detached {
		t.Fatal("ctrl-c did not detach")
	}
}

func TestModelLogSanitizationAndFailure(t *testing.T) {
	m := New([]string{"submit"}, 0, false, nil)
	m = update(m, LogLineMsg{Line: "\x1b[31mboom\x1b[0m"})
	m = update(m, DoneMsg{State: "FAILED"})
	if strings.Contains(m.View(), "\x1b") || !strings.Contains(m.View(), "boom") {
		t.Fatalf("unsafe log rendering: %q", m.View())
	}
}

func TestDoneQuits(t *testing.T) {
	m := New([]string{"submit"}, 0, false, nil)
	_, cmd := m.Update(DoneMsg{State: "DONE"})
	if cmd == nil {
		t.Fatal("DoneMsg did not request quit")
	}
}

func TestViewGoldenWidths(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  string
	}{
		{"wide", 120, "✔ manifest     case\n· cache        staged\n"},
		{"medium", 80, "✔ manifest     case\n· cache        staged\n"},
		{"compact", 50, "✔ manifest case\n· cache staged\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New([]string{"manifest", "cache"}, 0, false, nil)
			m = update(m, StageMsg{Name: "manifest", Status: "done", Detail: "case"})
			m = update(m, StageMsg{Name: "cache", Status: "pending", Detail: "staged"})
			m = update(m, tea.WindowSizeMsg{Width: tt.width})
			if got := m.View(); got != tt.want {
				t.Fatalf("View() = %q, want %q", got, tt.want)
			}
		})
	}
}

func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}
