package ui

import (
	"fmt"
	"io"
	"time"
)

type Progress struct {
	Out    io.Writer
	Term   Terminal
	Symbol Symbols
}

func (p Progress) Stage(name, detail string) {
	fmt.Fprintf(p.Out, "%s %-12s %s\n", p.Symbol.OK, name, detail)
}

func (p Progress) Notice(detail string) {
	fmt.Fprintf(p.Out, "%s\n", detail)
}

func (p Progress) Timestamped(name, detail string) {
	fmt.Fprintf(p.Out, "%s %s %-12s %s\n", time.Now().Format("15:04:05"), p.Symbol.OK, name, detail)
}
