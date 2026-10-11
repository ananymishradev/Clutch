// Package brand centralizes Sluice product names so a pure rename
// never scatters hardcoded strings across help, errors, and examples.
package brand

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// Name is the binary and command name.
	Name = "sluice"
	// DisplayName is the human-readable product name.
	DisplayName = "Sluice"
	// LegacyName is the pre-rename binary name. It appears only in
	// compatibility paths, migration docs, and the stale-name allowlist.
	LegacyName = "clutch"
	// LegacyDisplayName is the pre-rename product name.
	LegacyDisplayName = "Foam Clutch"

	// EnvPrefix is the new environment variable prefix.
	EnvPrefix = "SLUICE_"
	// LegacyEnvPrefix is the old prefix kept as fallback.
	LegacyEnvPrefix = "FOAM_CLUTCH_"

	// APIVersion is the canonical manifest apiVersion written by init.
	APIVersion = "sluice/v1alpha1"
	// LegacyAPIVersion is accepted indefinitely with a deprecation warning.
	LegacyAPIVersion = "foam-clutch/v1alpha1"

	// DefaultDBFile is the new default SQLite file basename.
	DefaultDBFile = "sluice.db"
	// LegacyDBFile is the old default kept as fallback. Never auto-migrated.
	LegacyDBFile = "foam-clutch.db"

	// RunScriptName is the per-run batch script for new runs only.
	RunScriptName = "sluice.sbatch"
	// LegacyRunScriptName is left untouched in existing run directories.
	LegacyRunScriptName = "foam-clutch.sbatch"

	// MetricPrefix is the new Prometheus metric prefix.
	MetricPrefix = "sluice_"
	// LegacyMetricPrefix is the removed prefix, documented in CHANGELOG.
	LegacyMetricPrefix = "foam_clutch_"
)

var warned = sync.Map{}

// Getenv implements new-then-legacy lookup for a bare suffix such as
// "API_TOKEN": it checks SLUICE_API_TOKEN first, then FOAM_CLUTCH_API_TOKEN.
// When only the legacy variable is set, it warns once per suffix on stderr.
// It never prints values.
func Getenv(suffix string) string {
	v, _ := LookupEnv(suffix)
	return v
}

// LookupEnv is Getenv with presence reporting, for callers that need to
// distinguish unset from empty (e.g. backend readiness checks).
func LookupEnv(suffix string) (string, bool) {
	if v, ok := os.LookupEnv(EnvPrefix + suffix); ok {
		return v, true
	}
	if v, ok := os.LookupEnv(LegacyEnvPrefix + suffix); ok {
		if _, done := warned.LoadOrStore(suffix, true); !done {
			fmt.Fprintf(os.Stderr, "warning: %s is deprecated; use %s instead\n",
				LegacyEnvPrefix+suffix, EnvPrefix+suffix)
		}
		return v, true
	}
	return "", false
}

// ResetWarnings clears the once-per-process warning state. Tests only.
func ResetWarnings() {
	warned = sync.Map{}
}

// DefaultDBPath returns the new default DB path for a manifest.
func DefaultDBPath(manifestPath string) string {
	return filepath.Join(filepath.Dir(manifestPath), DefaultDBFile)
}

// ResolveDBPath implements the DB fallback: explicit --db wins; otherwise
// <manifest-dir>/sluice.db is default. If it does not exist but
// <manifest-dir>/foam-clutch.db does, the legacy file is used with
// usedLegacy=true so callers print a one-time notice. It never moves,
// renames, copies, or deletes user data.
func ResolveDBPath(manifestPath, explicit string) (path string, usedLegacy bool) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, false
	}
	newPath := DefaultDBPath(manifestPath)
	legacyPath := filepath.Join(filepath.Dir(manifestPath), LegacyDBFile)
	if _, err := os.Stat(newPath); err == nil {
		return newPath, false
	}
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, true
	}
	return newPath, false
}

// LegacyDBNotice is the one-time notice printed when falling back.
func LegacyDBNotice(legacyPath string) string {
	return fmt.Sprintf("notice: using existing legacy database %s; new default is %s (data will not be moved automatically)", legacyPath, DefaultDBFile)
}
