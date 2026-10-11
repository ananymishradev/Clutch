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
