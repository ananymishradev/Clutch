package ui

import (
	"fmt"
	"io"
)

func Error(w io.Writer, title string, err error, hint string, color bool) {
	prefix := "error:"
	if color {
		prefix = Failure.Render("✖")
	}
	fmt.Fprintf(w, "%s %s\n", prefix, title)
	if err != nil {
		fmt.Fprintf(w, "\n  %s\n", err)
	}
	if hint != "" {
		fmt.Fprintf(w, "\n  hint: %s\n", hint)
	}
}
