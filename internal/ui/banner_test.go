package ui

import (
	"testing"
)

func TestBannerLinesAre35Columns(t *testing.T) {
	if len(logo) != 5 {
		t.Fatalf("logo has %d lines, want 5", len(logo))
	}
	for i, line := range logo {
		if n := len([]rune(line)); n != 35 {
			t.Fatalf("line %d is %d columns, want 35: %q", i, n, line)
		}
	}
}
