#!/bin/sh
set -eu

root=$(git rev-parse --show-toplevel)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/foam-clutch-mutations.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM

diff_file="$tmp/worktree.diff"
git diff --binary HEAD >"$diff_file"

copy_untracked() {
	git -C "$root" ls-files --others --exclude-standard |
		while IFS= read -r file; do
			mkdir -p "$tmp/wt/$(dirname "$file")"
			cp "$root/$file" "$tmp/wt/$file"
		done
}

run_mutation() {
	name=$1
	test_name=$2
	wt="$tmp/wt"
	rm -rf "$wt"
	git worktree add --detach "$wt" HEAD >/dev/null
	if [ -s "$diff_file" ]; then
		git -C "$wt" apply "$diff_file"
	fi
	copy_untracked
	if git -C "$wt" apply "$root/testdata/mutations/$name.patch" >/dev/null 2>&1 &&
		(cd "$wt" && go test ./cmd/clutch -run "^${test_name}$" -count=1 >/dev/null 2>&1); then
		printf '%-28s %-42s NO\n' "$name" "$test_name"
		git worktree remove --force "$wt" >/dev/null 2>&1 || true
		return 1
	fi
	printf '%-28s %-42s YES\n' "$name" "$test_name"
	git worktree remove --force "$wt" >/dev/null 2>&1 || true
}

printf '%-28s %-42s %s\n' mutation actual-test caught
failed=0
run_mutation a-spinner TestPipe_RunNoWatchMatchesBaseline || failed=1
run_mutation b-ansi TestPipe_RunNoWatchMatchesBaseline || failed=1
run_mutation c-banner TestPipe_RunNoWatchMatchesBaseline || failed=1
run_mutation d-failed-zero TestPipeExitCodes || failed=1
run_mutation e-logs-decoration TestPipe_LogsPathsOnly || failed=1
run_mutation f-progress-stdout TestPipe_RunNoWatchMatchesBaseline || failed=1
run_mutation g-drop-preflight TestPipe_RunPreflightFailureReportsFindings || failed=1
run_mutation h-json-live TestPTY_RunJSONDoesNotStartLiveView || failed=1
run_mutation i-detach-cancel TestPTY_DetachWithCtrlCAndQ || failed=1
run_mutation j-cursor-show TestPTY_TerminalRestoredAndNoAltScreen || failed=1
exit "$failed"
