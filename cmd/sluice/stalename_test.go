package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaleNameGuard fails when the case-insensitive string "clutch"
// appears in any tracked file outside the explicit allowlist below.
// Reviewed allowlist: CHANGELOG history, the README migration section,
// the compat layer and its tests, installer legacy handling, and the
// pre-rename golden generator.
func TestStaleNameGuard(t *testing.T) {
	allowedFiles := map[string]bool{
		"CHANGELOG.md":                          true,
		".gitignore":                            true, // keeps old /clutch + /foam-clutch entries
		"install.sh":                            true,
		"uninstall.sh":                          true,
		"internal/brand/brand.go":               true,
		"internal/brand/brand_test.go":          true,
		"internal/manifest/compat_test.go":      true,
		"internal/doctor/checks_legacy.go":      true,
		"internal/doctor/checks_legacy_test.go": true,
		"cmd/sluice/uninstall_test.go":          true,
		"cmd/sluice/stalename_test.go":          true,
		"testdata/golden/regen.sh":              true,
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("git unavailable")
	}
	root := strings.TrimSpace(string(out))
	filesOut, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Skip("git ls-files unavailable")
	}
	for _, file := range strings.Split(strings.TrimSpace(string(filesOut)), "\n") {
		if file == "" {
			continue
		}
		// Binary release artifacts are untracked or gitignored; skip by name.
		if strings.HasPrefix(file, "sluice-") || strings.HasPrefix(file, "clutch-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(data))
		if !strings.Contains(lower, "clutch") {
			continue
		}
		// Pre-rename golden fixtures stay until testdata/golden/rename.sh
		// transforms them per the documented rules.
		if strings.HasPrefix(file, "testdata/golden/") {
			continue
		}
		if allowedFiles[file] {
			continue
		}
		if file == "README.md" {
			// Only the migration section may mention the old name.
			allowed := true
			for _, line := range strings.Split(string(data), "\n") {
				ll := strings.ToLower(line)
				if !strings.Contains(ll, "clutch") {
					continue
				}
				if strings.Contains(ll, "formerly") || strings.Contains(ll, "migrat") ||
					strings.Contains(ll, "foam clutch") || strings.Contains(ll, "foam-clutch") ||
					strings.Contains(ll, "foam_clutch") || strings.Contains(ll, "`clutch`") ||
					strings.Contains(ll, "legacy") {
					continue
				}
				allowed = false
				t.Errorf("README.md non-migration clutch reference: %q", line)
			}
			if allowed {
				continue
			}
			continue
		}
		t.Errorf("stale name 'clutch' in tracked file %s (add to allowlist only with review)", file)
	}
}
