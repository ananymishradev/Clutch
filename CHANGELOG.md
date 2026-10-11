# Changelog

## Unreleased — Rename Foam Clutch to Sluice

Pure rename: binary `clutch` becomes `sluice` (module
`github.com/ananymishradev/sluice`). No behavior changes except the names
below. The `it` alias for `run` stays. `.foam-cache/` and `.foam-runs/` keep
their names.

- Manifest `apiVersion`: new canonical `sluice/v1alpha1`; `sluice init`
  writes the new value. `foam-clutch/v1alpha1` is still accepted with a
  one-line deprecation warning on stderr.
- Env vars: new `SLUICE_*` names with fallback to `FOAM_CLUTCH_*`. New wins
  when both are set; a once-per-process warning prints when only the old name
  is used. Values are never logged.
- DB file: new default `sluice.db` with fallback to an existing
  `<manifest-dir>/foam-clutch.db` plus a one-time notice. User data is never
  moved, renamed, copied, or deleted automatically.
- Run script: new runs write `sluice.sbatch`; existing run directories are
  untouched.
- **Breaking:** Prometheus metrics `foam_clutch_*` are now `sluice_*` with no
  dual emission. Update dashboards and alerts.
- No `clutch` shim binary. The installer only prints a notice if it finds an
  old copy.
