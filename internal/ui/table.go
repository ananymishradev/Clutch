package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ananymishradev/sluice/internal/store"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func JobsTable(w io.Writer, jobs []store.Job, color bool) {
	rows := make([][]string, 0, len(jobs))
	for _, j := range jobs {
		id := j.ID
		if len(id) > 8 {
			id = id[:8]
		}
		state := j.State
		if color {
			state = State(j.State).Render(state)
		}
		rows = append(rows, []string{id, j.Name, state, fmt.Sprint(j.SlurmID), "-", "-", Relative(j.UpdatedAt)})
	}
	t := table.New().
		Headers("ID", "NAME", "STATE", "SLURM", "NODES", "ELAPSED", "UPDATED").
		Rows(rows...).
		Border(lipgloss.HiddenBorder()).
		BorderRow(false).
		BorderColumn(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				if color {
					return Dim
				}
				return lipgloss.NewStyle()
			}
			if col == 2 && color {
				return State(jobs[row].State)
			}
			return lipgloss.NewStyle()
		})
	_, _ = io.WriteString(w, t.String()+"\n")
}

func JobJSON(j store.Job) map[string]any {
	return map[string]any{
		"id": j.ID, "name": j.Name, "state": j.State, "manifest_path": j.ManifestPath,
		"case_hash": j.CaseHash, "run_path": j.RunPath, "cache_path": j.CachePath,
		"slurm_id": j.SlurmID, "error": j.Error, "updated_at": j.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func Trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 4 {
		return strings.Repeat(".", n)
	}
	return s[:n-3] + "..."
}
