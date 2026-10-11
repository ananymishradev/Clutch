package ui

import "github.com/charmbracelet/lipgloss"

var (
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	Failure = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	Info    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	Dim     = lipgloss.NewStyle().Faint(true)
)

func State(state string) lipgloss.Style {
	switch state {
	case "QUEUED":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	case "RUNNING":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	case "DONE":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	case "FAILED":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	case "REQUEUED":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#D946EF"))
	case "UNKNOWN":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#7F1D1D"))
	default:
		return Dim
	}
}
