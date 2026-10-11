# Changelog

## Unreleased

- Manifest `apiVersion` is `sluice/v1alpha1`; `sluice init` writes it.
- Env vars use the `SLUICE_*` names (e.g. `SLUICE_API_TOKEN`). Values are
  never logged.
- DB default file is `sluice.db` next to the manifest. User data is never
  moved, renamed, copied, or deleted automatically.
- New runs write `sluice.sbatch`; existing run directories are untouched.
- Prometheus metrics use the `sluice_*` prefix with no dual emission.
  Point dashboards and alerts at those names.
- The `it` alias for `run` stays. `.foam-cache/` and `.foam-runs/` keep
  their names. The installer never deletes other copies it finds.
