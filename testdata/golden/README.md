# Non-TTY output goldens

These files were generated from the pre-live-view commit `0e51cf9`
(`Added Native Cobra port of existing commands`), the last commit before
`ea86e76` (`Live Bubbletea view for run`). The baseline binary was built in a
detached worktree and executed with stdout and stderr connected to pipes.

To deliberately regenerate them, provide the old commit explicitly:

```sh
BASELINE_SHA=0e51cf9 ./testdata/golden/regen.sh
```

The script refuses an unset SHA and refuses to run when `BASELINE_SHA` equals
the current `HEAD`. It creates a temporary detached worktree and uses the
repository's POSIX fake Slurm commands. Do not regenerate these files from
the current tree when investigating a parity regression.

The scenarios cover successful, failed, cancelled, dry-run, and no-watch
runs; successful and failing validation in table and JSON modes; empty and
populated lists; refreshed and JSON status; path-only logs; cancel; and doctor
output. Timestamps, runtime IDs, hashes, temporary paths, and table trailing
whitespace are normalized. Run the generator twice with the same
`BASELINE_SHA` after changing normalization to verify determinism.

## Post-rename transform

After the Foam Clutch to Sluice rename, `testdata/golden/rename.sh`
transforms these baseline files in place (the baseline itself is never
regenerated from the new code). Rules, longest tokens first:

1. `foam-clutch/v1alpha1` -> `sluice/v1alpha1` (manifest apiVersion)
2. `foam-clutch.sbatch` -> `sluice.sbatch` (per-run script; no-op in the
   current goldens because run paths are normalized to `<TEMP_PATH>`)
3. `foam-clutch.db` -> `sluice.db` (default DB file; no-op for the same
   normalization reason)
4. `foam-clutch job:` -> `sluice job:` (rendered batch echo)
5. `clutch run backend=` -> `sluice run backend=` (plain run log prefix)
6. `run \`clutch --help\`` -> `run \`sluice --help\`` (structured error hint)
7. `Try: clutch run` -> `Try: sluice run` (list-empty hint)

Every changed line in `git diff testdata/golden/` is explained by one of
these rules. A test failure not explained by them is a behavior change:
stop and report it instead of editing the golden.
