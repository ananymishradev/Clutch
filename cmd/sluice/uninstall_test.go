package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestUninstallAllKeepsUnrelatedClutch ensures --all never deletes a clutch
// binary whose version output does not identify it as Foam Clutch.
func TestUninstallAllKeepsUnrelatedClutch(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o750); err != nil {
		t.Fatal(err)
	}
	// Installed sluice to be removed.
	if err := os.WriteFile(filepath.Join(localBin, "sluice"), []byte("#!/bin/sh\necho sluice\n"), 0o750); err != nil {
		t.Fatal(err)
	}
	// Unrelated clutch binary: version output has no Foam Clutch markers.
	unrelated := "#!/bin/sh\necho 'totally different tool v1'\n"
	if err := os.WriteFile(filepath.Join(localBin, "clutch"), []byte(unrelated), 0o750); err != nil {
		t.Fatal(err)
	}
	gopath := t.TempDir()
	cmd := exec.Command("bash", filepath.Join(root, "uninstall.sh"), "--all")
	cmd.Env = append(os.Environ(), "HOME="+home, "GOPATH="+gopath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil && !containsStr(string(out), "uninstall done") {
		t.Fatalf("uninstall failed: %v output=%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(localBin, "sluice")); !os.IsNotExist(err) {
		t.Fatal("sluice was not removed")
	}
	if _, err := os.Stat(filepath.Join(localBin, "clutch")); err != nil {
		t.Fatalf("unrelated clutch was removed: %v output=%s", err, out)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
