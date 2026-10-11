package testutil

import "testing"

func TestNormalizePreservesMeaningfulDifferences(t *testing.T) {
	a := Normalize("state: RUNNING local 12345678")
	b := Normalize("state: FAILED local 12345678")
	if a == b {
		t.Fatal("normalization erased a meaningful state difference")
	}
}

func TestNormalizeReplacesRuntimeValues(t *testing.T) {
	got := Normalize("2026/10/11 10:00:00 local 12345678 hash abcdef0123456789abcdef0123456789abcdef01 /tmp/case-1/runs")
	for _, want := range []string{"<TIMESTAMP>", "<ID>", "<CASE_HASH>", "<TEMP_PATH>"} {
		if !contains(got, want) {
			t.Fatalf("Normalize(%q) = %q, missing %s", got, want, want)
		}
	}
}

func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
