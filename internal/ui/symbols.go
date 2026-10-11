package ui

type Symbols struct {
	OK, Fail, Warn, Run, Skip string
}

func SymbolsFor(t Terminal) Symbols {
	if t.TTY && t.UTF8() {
		return Symbols{"✔", "✖", "⚠", "●", "○"}
	}
	return Symbols{"[ok]", "[fail]", "[warn]", "[run]", "[skip]"}
}
