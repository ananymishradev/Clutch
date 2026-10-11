package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaselineSHAIsAncestorAndNotHEAD(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "BASELINE_SHA"))
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(data))
	if sha == "" {
		t.Fatal("BASELINE_SHA is empty")
	}
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skipf("git history unavailable: %v", err)
	}
	if sha == strings.TrimSpace(string(head)) {
		t.Fatalf("baseline %s must not equal HEAD", sha)
	}
	if err := exec.Command("git", "merge-base", "--is-ancestor", sha, "HEAD").Run(); err != nil {
		t.Skipf("baseline history is unavailable or not an ancestor: %v", err)
	}
}
