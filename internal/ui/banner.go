package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var logo = []string{
	` ▓▓▓  ▓     ▓   ▓ ▓▓▓▓▓  ▓▓▓  ▓▓▓▓▓`,
	`▓     ▓     ▓   ▓   ▓   ▓     ▓    `,
	` ▓▓▓  ▓     ▓   ▓   ▓   ▓     ▓▓▓▓ `,
	`    ▓ ▓     ▓   ▓   ▓   ▓     ▓    `,
	` ▓▓▓  ▓▓▓▓▓  ▓▓▓  ▓▓▓▓▓  ▓▓▓  ▓▓▓▓▓`,
}

var gradient = []string{"#22D3EE", "#38BDF8", "#60A5FA", "#818CF8", "#A78BFA"}

func Banner(version string, color bool) string {
	var b strings.Builder
	for i, line := range logo {
		style := lipgloss.NewStyle()
		if color {
			style = style.Foreground(lipgloss.Color(gradient[i]))
		}
		b.WriteString(style.Render(line))
		b.WriteByte('\n')
	}
	tag := lipgloss.NewStyle()
	if color {
		tag = tag.Faint(true)
	}
	fmt.Fprintf(&b, "%s\n", tag.Render("OpenFOAM × Slurm, without the babysitting · "+version))
	return b.String()
}
