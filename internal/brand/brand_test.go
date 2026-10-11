package brand

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetenvPrecedence(t *testing.T) {
	ResetWarnings()
	t.Setenv(EnvPrefix+"API_TOKEN", "new-value")
	t.Setenv(LegacyEnvPrefix+"API_TOKEN", "old-value")
	if got := Getenv("API_TOKEN"); got != "new-value" {
		t.Fatalf("new wins, got %q", got)
	}
}

func TestGetenvLegacyWarnsOnceWithoutValues(t *testing.T) {
	ResetWarnings()
	os.Unsetenv(EnvPrefix + "API_TOKEN")
	t.Setenv(LegacyEnvPrefix+"API_TOKEN", "old-secret-value")
	// Capture stderr by redirecting os.Stderr to a pipe.
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	Getenv("API_TOKEN")
	Getenv("API_TOKEN")
	_ = w.Close()
	os.Stderr = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	count := 0
	for i := 0; i+7 <= len(out); i++ {
		if out[i:i+7] == "warning" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("warning emitted %d times, want exactly once; output=%q", count, out)
	}
	for _, secret := range []string{"old-secret-value"} {
		if len(secret) > 0 && contains(out, secret) {
			t.Fatalf("value leaked in warning: %q", out)
		}
	}
	if !contains(out, LegacyEnvPrefix+"API_TOKEN") || !contains(out, EnvPrefix+"API_TOKEN") {
		t.Fatalf("warning must name both vars; output=%q", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestResolveDBMatrix(t *testing.T) {
	mk := func(files ...string) string {
		dir := t.TempDir()
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "m.yaml")
	}
	// Neither: new default, nothing moved.
	m := mk()
	got, legacy := ResolveDBPath(m, "")
	if got != DefaultDBPath(m) || legacy {
		t.Fatalf("neither: got %q legacy=%v", got, legacy)
	}
	// Old only: legacy fallback.
	m = mk(LegacyDBFile)
	got, legacy = ResolveDBPath(m, "")
	if !legacy || filepath.Base(got) != LegacyDBFile {
		t.Fatalf("old only: got %q legacy=%v", got, legacy)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m), DefaultDBFile)); !os.IsNotExist(err) {
		t.Fatal("new file must not be created")
	}
	// New only.
	m = mk(DefaultDBFile)
	got, legacy = ResolveDBPath(m, "")
	if legacy || filepath.Base(got) != DefaultDBFile {
		t.Fatalf("new only: got %q legacy=%v", got, legacy)
	}
	// Both: new wins.
	m = mk(DefaultDBFile, LegacyDBFile)
	got, legacy = ResolveDBPath(m, "")
	if legacy || filepath.Base(got) != DefaultDBFile {
		t.Fatalf("both: got %q legacy=%v", got, legacy)
	}
	// Explicit wins over both.
	got, legacy = ResolveDBPath(m, "/tmp/custom.db")
	if got != "/tmp/custom.db" || legacy {
		t.Fatalf("explicit: got %q legacy=%v", got, legacy)
	}
	// Assert nothing was moved or deleted: both files still exist.
	for _, f := range []string{DefaultDBFile, LegacyDBFile} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(m), f)); err != nil {
			t.Fatalf("file %s missing after resolve: %v", f, err)
		}
	}
}
