package ui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

type Terminal struct {
	Color bool
	TTY   bool
}

func Detect(noColor bool) Terminal {
	tty := term.IsTerminal(int(os.Stderr.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	color := tty && !noColor && os.Getenv("NO_COLOR") == ""
	return Terminal{Color: color, TTY: tty}
}

func (t Terminal) UTF8() bool {
	locale := strings.ToLower(os.Getenv("LC_ALL") + " " + os.Getenv("LC_CTYPE") + " " + os.Getenv("LANG"))
	return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
}
