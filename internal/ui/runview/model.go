package runview

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/ananymishradev/sluice/internal/ui"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StageStatus string

const (
	Pending StageStatus = "pending"
	Active  StageStatus = "active"
	Done    StageStatus = "done"
	Failed  StageStatus = "failed"
	Skipped StageStatus = "skipped"
)

type StageRow struct {
	Name    string
	Status  StageStatus
	Detail  string
	Started time.Time
	Elapsed time.Duration
}

type StageMsg struct{ Name, Status, Detail string }
type StateMsg struct {
	State   string
	SlurmID int
}
type LogLineMsg struct{ Line string }
type DoneMsg struct {
	State string
	Err   error
}
type DetachMsg struct{}
type tickMsg time.Time

type Model struct {
	Rows              []StageRow
	Spinner           spinner.Model
	JobState          string
	SlurmID           int
	CheckpointSeconds int
	Started           time.Time
	Now               time.Time
	Width             int
	LogLines          []string
	Done              bool
	FinalState        string
	Err               error
	Detached          bool
	OnDetach          func()
	Color             bool
}

func New(stages []string, checkpointSeconds int, color bool, onDetach func()) Model {
	rows := make([]StageRow, 0, len(stages))
	for _, name := range stages {
		rows = append(rows, StageRow{Name: name, Status: Pending})
	}
	s := spinner.New()
	s.Spinner = spinner.Dot
	now := time.Now()
	return Model{Rows: rows, Spinner: s, CheckpointSeconds: checkpointSeconds,
		Started: now, Now: now, Color: color, OnDetach: onDetach}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.detach()
			return m, tea.Quit
		}
	case DetachMsg:
		m.detach()
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.Width = msg.Width
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case tickMsg:
		m.Now = time.Time(msg)
		if !m.Done {
			return m, tickCmd()
		}
	case StageMsg:
		m.applyStage(msg)
	case StateMsg:
		if msg.State == "RUNNING" && m.JobState != "RUNNING" {
			m.Started = m.Now
		}
		m.JobState, m.SlurmID = msg.State, msg.SlurmID
	case LogLineMsg:
		m.LogLines = append(m.LogLines, cleanLog(msg.Line))
		if len(m.LogLines) > 8 {
			m.LogLines = m.LogLines[len(m.LogLines)-8:]
		}
	case DoneMsg:
		m.Done, m.FinalState, m.Err, m.Now = true, msg.State, msg.Err, time.Now()
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) detach() {
	m.Detached = true
	if m.OnDetach != nil {
		m.OnDetach()
	}
}

func (m *Model) applyStage(msg StageMsg) {
	for i := range m.Rows {
		if m.Rows[i].Name != msg.Name {
			continue
		}
		m.Rows[i].Detail = msg.Detail
		switch msg.Status {
		case "started":
			m.Rows[i].Status, m.Rows[i].Started = Active, m.Now
		case "done":
			m.Rows[i].Status, m.Rows[i].Elapsed = Done, m.Now.Sub(m.Rows[i].Started)
		case "failed":
			m.Rows[i].Status, m.Rows[i].Elapsed = Failed, m.Now.Sub(m.Rows[i].Started)
		default:
			m.Rows[i].Status = StageStatus(msg.Status)
		}
		return
	}
}

func (m Model) View() string {
	var b strings.Builder
	width := m.Width
	if width <= 0 {
		width = 80
	}
	compact := width < 60
	for _, row := range m.Rows {
		symbol := "·"
		switch row.Status {
		case Active:
			symbol = m.Spinner.View()
		case Done:
			symbol = "✔"
		case Failed:
			symbol = "✖"
		case Skipped:
			symbol = "—"
		}
		if compact {
			b.WriteString(fmt.Sprintf("%s %s\n", symbol, truncate(row.Name+" "+row.Detail, width-4)))
			continue
		}
		remaining := width - lipgloss.Width(symbol) - lipgloss.Width(row.Name) - 5
		b.WriteString(fmt.Sprintf("%s %-12s %s\n", symbol, row.Name, truncate(row.Detail, remaining)))
	}
	if m.JobState == "RUNNING" || m.JobState == "QUEUED" || m.JobState == "REQUEUED" {
		elapsed := m.Now.Sub(m.Started)
		if elapsed < 0 {
			elapsed = 0
		}
		if compact {
			b.WriteString(fmt.Sprintf("\n%s %s\n", m.JobState, ui.Duration(elapsed)))
		} else {
			b.WriteString(fmt.Sprintf("\nphase: %s  elapsed: %s  slurm: %d\n", m.JobState, ui.Duration(elapsed), m.SlurmID))
		}
		if m.CheckpointSeconds > 0 {
			b.WriteString(fmt.Sprintf("checkpoint armed at T-%ds\n", m.CheckpointSeconds))
		}
	}
	if len(m.LogLines) > 0 {
		body := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render(strings.Join(m.LogLines, "\n"))
		b.WriteString(body + "\n")
	}
	if m.Done {
		b.WriteString(m.FinalState + "\n")
		if m.Err != nil {
			b.WriteString("error: " + m.Err.Error() + "\n")
		}
	}
	return b.String()
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func cleanLog(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || (unicode.IsPrint(r) && r != '\r') {
			b.WriteRune(r)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
