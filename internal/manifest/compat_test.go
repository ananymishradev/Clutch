package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ananymishradev/sluice/internal/brand"
)

func writeManifest(t *testing.T, apiVersion string) string {
	t.Helper()
	dir := t.TempDir()
	caseDir := filepath.Join(dir, "case")
	for _, d := range []string{"system", "constant", "0"} {
		if err := os.MkdirAll(filepath.Join(caseDir, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(caseDir, "system", "controlDict"), []byte("application simpleFoam;\n"), 0o640)
	os.WriteFile(filepath.Join(caseDir, "system", "blockMeshDict"), []byte("blocks ();\n"), 0o640)
	os.WriteFile(filepath.Join(caseDir, "0", "p"), []byte("x\n"), 0o640)
	os.WriteFile(filepath.Join(caseDir, "constant", "transportProperties"), []byte("x\n"), 0o640)
	p := filepath.Join(dir, "m.yaml")
	doc := "apiVersion: " + apiVersion + "\nname: t\ncase:\n  path: " + caseDir + "\nresources:\n  nodes: 1\n  tasksPerNode: 1\n  timeLimitMinutes: 5\nsolver:\n  name: simpleFoam\nruntime:\n  foamBashrc: /etc/bash.bashrc\n"
	if err := os.WriteFile(p, []byte(doc), 0o640); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLegacyAPIVersionAcceptedWithWarning(t *testing.T) {
	p := writeManifest(t, brand.LegacyAPIVersion)
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	m, err := Load(p)
	_ = w.Close()
	os.Stderr = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if err != nil {
		t.Fatalf("legacy apiVersion rejected: %v", err)
	}
	if m.APIVersion != brand.LegacyAPIVersion {
		t.Fatalf("APIVersion = %q", m.APIVersion)
	}
	if !strings.Contains(out, "deprecated") || !strings.Contains(out, brand.APIVersion) {
		t.Fatalf("missing deprecation warning: %q", out)
	}
}

func TestNewAPIVersionIsDefault(t *testing.T) {
	p := writeManifest(t, brand.APIVersion)
	m, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.APIVersion != brand.APIVersion {
		t.Fatalf("APIVersion = %q", m.APIVersion)
	}
}

func TestUnknownAPIVersionRejectedListingBoth(t *testing.T) {
	p := writeManifest(t, "other/v9")
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), brand.APIVersion) || !strings.Contains(err.Error(), brand.LegacyAPIVersion) {
		t.Fatalf("error must list both values: %v", err)
	}
}
