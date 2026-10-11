#!/bin/sh
set -eu

: "${BASELINE_SHA:?set BASELINE_SHA to the pre-live-view commit}"
head=$(git rev-parse HEAD)
[ "$BASELINE_SHA" != "$head" ] || {
  echo "BASELINE_SHA must not equal HEAD" >&2
  exit 2
}

root=$(git rev-parse --show-toplevel)
out="$root/testdata/golden"
wt=$(mktemp -d "${TMPDIR:-/tmp}/foam-clutch-baseline.XXXXXX")
case_root=$(mktemp -d "${TMPDIR:-/tmp}/foam-clutch-golden.XXXXXX")
trap 'git worktree remove --force "$wt" >/dev/null 2>&1 || true; rm -rf "$case_root"' EXIT INT TERM

git worktree add --detach "$wt" "$BASELINE_SHA" >/dev/null
mkdir -p "$wt/testdata/fakeslurm"
cp "$root"/testdata/fakeslurm/* "$wt/testdata/fakeslurm/"
chmod +x "$wt"/testdata/fakeslurm/*
(cd "$wt" && go build -o "$wt/clutch" ./cmd/clutch)

case_dir="$case_root/case"
mkdir -p "$case_dir/system" "$case_dir/constant" "$case_dir/0"
printf 'application simpleFoam;\nrunTimeModifiable true;\n' > "$case_dir/system/controlDict"
printf 'blocks ();\n' > "$case_dir/system/blockMeshDict"
printf 'internalField uniform 0;\n' > "$case_dir/0/p"
printf 'nu 1e-05;\n' > "$case_dir/constant/transportProperties"
cat > "$case_root/m.yaml" <<EOF
apiVersion: foam-clutch/v1alpha1
name: pty-case
case:
  path: $case_dir
resources:
  nodes: 1
  tasksPerNode: 1
  timeLimitMinutes: 60
solver:
  name: simpleFoam
  checkpointSeconds: 600
mesh:
  strategy: blockMesh
runtime:
  foamBashrc: /etc/bash.bashrc
storage:
  cacheDir: $case_root/cache
  runDir: $case_root/runs
EOF

normalize() {
  sed -E \
    -e 's@[0-9]{4}/[0-9]{2}/[0-9]{2} [0-9:.]+@<TIMESTAMP>@g' \
    -e 's@[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z@<TIMESTAMP>@g' \
    -e 's@[0-9]{2}:[0-9]{2}:[0-9]{2}@<TIMESTAMP>@g' \
    -e 's@[0-9a-fA-F]{8}-[0-9a-fA-F-]{27}@<LOCAL_ID>@g' \
    -e 's@[0-9a-fA-F]{40,64}@<CASE_HASH>@g' \
    -e 's@(^|[^0-9a-fA-F])[0-9a-fA-F]{8}([^0-9a-fA-F]|$)@\1<ID>\2@g' \
    -e 's@/tmp/[A-Za-z0-9._/-]+@<TEMP_PATH>@g' \
    -e 's@([Ss]lurm( job| ID)?[=: ]+)[0-9]+@\1<SLURM_ID>@g' \
    -e 's@<TEMP_PATH><CASE_HASH>@<TEMP_PATH>/<CASE_HASH>@g' \
    -e 's@<TEMP_PATH><LOCAL_ID>@<TEMP_PATH>/<LOCAL_ID>@g' \
    -e 's@[[:space:]]+$@@'
}
run_one() {
  name=$1
  shift
  fake="$case_root/fake-$name"
  mkdir -p "$fake"
  printf 'COMPLETED\n' > "$fake/state"
  PATH="$wt/testdata/fakeslurm:$PATH" FAKESLURM_DIR="$fake" \
    "$wt/clutch" "$@" > "$out/$name.stdout.raw" 2> "$out/$name.stderr.raw" || true
  normalize < "$out/$name.stdout.raw" > "$out/$name.stdout"
  normalize < "$out/$name.stderr.raw" > "$out/$name.stderr"
  rm -f "$out/$name.stdout.raw" "$out/$name.stderr.raw"
}
run_one_state() {
  name=$1
  state=$2
  shift 2
  fake="$case_root/fake-$name"
  mkdir -p "$fake"
  printf '%s\n' "$state" > "$fake/state"
  PATH="$wt/testdata/fakeslurm:$PATH" FAKESLURM_DIR="$fake" \
    "$wt/clutch" "$@" > "$out/$name.stdout.raw" 2> "$out/$name.stderr.raw" || true
  normalize < "$out/$name.stdout.raw" > "$out/$name.stdout"
  normalize < "$out/$name.stderr.raw" > "$out/$name.stderr"
  rm -f "$out/$name.stdout.raw" "$out/$name.stderr.raw"
}

run_one run-done run --manifest "$case_root/m.yaml" --db "$case_root/jobs.db" --backend cli --no-watch
run_one run-dry-run run --manifest "$case_root/m.yaml" --db "$case_root/dry.db" --backend cli --dry-run
run_one run-no-watch run --manifest "$case_root/m.yaml" --db "$case_root/no-watch.db" --backend cli --no-watch
run_one_state run-failed FAILED run --manifest "$case_root/m.yaml" --db "$case_root/failed.db" --backend cli --interval 1ms
run_one_state run-cancelled CANCELLED run --manifest "$case_root/m.yaml" --db "$case_root/cancelled.db" --backend cli --interval 1ms
run_one validate-success validate --manifest "$case_root/m.yaml"
run_one validate-json validate --manifest "$case_root/m.yaml" --output json
cp "$case_dir/system/controlDict" "$case_root/controlDict.ok"
printf '#codeStream\n' > "$case_dir/system/controlDict"
run_one validate-preflight-failure validate --manifest "$case_root/m.yaml"
run_one validate-preflight-failure-json validate --manifest "$case_root/m.yaml" --output json
mv "$case_root/controlDict.ok" "$case_dir/system/controlDict"
run_one list-empty list --db "$case_root/empty.db"
run_one list-empty-json list --db "$case_root/empty-json.db" --output json
run_one list-populated list --db "$case_root/jobs.db"
run_one list-populated-json list --db "$case_root/jobs.db" --output json
run_one status status --db "$case_root/jobs.db" --id "$(sqlite3 "$case_root/jobs.db" 'select id from jobs limit 1')"
run_one status-refresh status --db "$case_root/jobs.db" --id "$(sqlite3 "$case_root/jobs.db" 'select id from jobs limit 1')" --refresh --backend cli
run_one status-json status --db "$case_root/jobs.db" --id "$(sqlite3 "$case_root/jobs.db" 'select id from jobs limit 1')" --output json
run_one logs logs --db "$case_root/jobs.db" --latest --follow=false
run_one cancel cancel --db "$case_root/jobs.db" --slurm 48213 --yes
run_one doctor doctor --db "$case_root/jobs.db"
