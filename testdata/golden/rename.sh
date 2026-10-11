#!/bin/sh
# rename.sh - transform pre-rename baseline goldens to post-rename text.
# The goldens were generated from the pre-rename baseline commit; do NOT
# regenerate them from the new code. This script applies a documented,
# minimal set of substitution rules so parity tests compare against the
# transformed goldens. If any test fails for a reason NOT explained by
# these rules, that is a behavior change: stop and report it instead of
# editing the golden.
set -eu

root=$(git rev-parse --show-toplevel)
dir="$root/testdata/golden"

# Rules (longest tokens first):
#  1. foam-clutch/v1alpha1 -> sluice/v1alpha1 (manifest apiVersion)
#  2. foam-clutch.sbatch   -> sluice.sbatch   (per-run script, new runs)
#  3. foam-clutch.db       -> sluice.db       (default DB file)
#  4. foam-clutch job:     -> sluice job:     (rendered batch echo)
#  5. clutch run backend=  -> sluice run backend= (plain run log prefix)
#  6. run `clutch --help`  -> run `sluice --help` (structured error hint)
#  7. Try: clutch run      -> Try: sluice run      (list-empty hint)
for f in "$dir"/*.stdout "$dir"/*.stderr; do
  [ -e "$f" ] || continue
  sed -i \
    -e 's|foam-clutch/v1alpha1|sluice/v1alpha1|g' \
    -e 's|foam-clutch\.sbatch|sluice.sbatch|g' \
    -e 's|foam-clutch\.db|sluice.db|g' \
    -e 's|foam-clutch job:|sluice job:|g' \
    -e 's|clutch run backend=|sluice run backend=|g' \
    -e 's|run `clutch --help`|run `sluice --help`|g' \
    -e 's|Try: clutch run|Try: sluice run|g' \
    "$f"
done
echo "transformed goldens in $dir"
