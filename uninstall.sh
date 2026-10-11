#!/usr/bin/env bash
# uninstall.sh - remove the `sluice` CLI binary.
# Usage:
#   ./uninstall.sh [--prefix ~/.local] [--bindir <dir>] [--system] [--all] [-h|--help]
#
# Default removes $PREFIX/bin/sluice (i.e. ~/.local/bin/sluice).
#   --system == --bindir /usr/local/bin
#   --all    == also scan GOPATH/bin, ~/.local/bin, /usr/local/bin for sluice,
#              plus a legacy `clutch` ONLY IF its `version` output identifies
#              it as Foam Clutch (never delete an unrelated `clutch` binary).
# Never touches databases, .foam-cache/, or .foam-runs/ (user data).
set -euo pipefail

PREFIX="${PREFIX:-$HOME/.local}"
BINDIR=""
SYSTEM=0
ALL=0

usage() {
  sed -n '2,10p' "$0"
}

# is_legacy_clutch <path>: true only when `path version` output identifies
# the binary as Foam Clutch. Never identifies by filename alone.
is_legacy_clutch() {
  local bin="$1" out
  [ -x "$bin" ] || return 1
  out="$("$bin" version 2>&1 || true)"
  case "$out" in
    *"Foam Clutch"*|*"foam-clutch"*) return 0 ;;
  esac
  case "$out" in
    *"sluice"*|*"Sluice"*) return 1 ;;
  esac
  case "$out" in
    clutch\ *|clutch) return 0 ;;
  esac
  return 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a value}"; shift 2 ;;
    --bindir) BINDIR="${2:?--bindir needs a value}"; shift 2 ;;
    --system) SYSTEM=1; shift ;;
    --all) ALL=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$SYSTEM" -eq 1 ]; then
  BINDIR="/usr/local/bin"
elif [ -z "$BINDIR" ]; then
  BINDIR="$PREFIX/bin"
fi

targets=("$BINDIR/sluice")
if [ "$ALL" -eq 1 ]; then
  GOPATH_BIN="$(go env GOPATH 2>/dev/null || true)/bin/sluice"
  for c in "$HOME/.local/bin/sluice" "$GOPATH_BIN" "/usr/local/bin/sluice"; do
    skip=0
    for t in "${targets[@]}"; do [ "$t" = "$c" ] && skip=1; done
    [ "$skip" -eq 0 ] && targets+=("$c")
  done
fi

removed=0
for target in "${targets[@]}"; do
  [ -z "$target" ] || [ "$target" = "/bin/sluice" ] && continue
  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ -w "$(dirname "$target")" ]; then
      rm -f "$target"
    else
      echo "need write access to $(dirname "$target"), retrying with sudo"
      sudo rm -f "$target"
    fi
    echo "removed $target"
    removed=1
  else
    echo "not found: $target"
  fi
done

# Legacy handling under --all only, with version-output identification.
if [ "$ALL" -eq 1 ]; then
  GOPATH_CLUTCH="$(go env GOPATH 2>/dev/null || true)/bin/clutch"
  for legacy in "$HOME/.local/bin/clutch" "$GOPATH_CLUTCH" "/usr/local/bin/clutch"; do
    [ -e "$legacy" ] || [ -L "$legacy" ] || continue
    if is_legacy_clutch "$legacy"; then
      if [ -w "$(dirname "$legacy")" ]; then
        rm -f "$legacy"
      else
        echo "need write access to $(dirname "$legacy"), retrying with sudo"
        sudo rm -f "$legacy"
      fi
      echo "removed legacy Foam Clutch $legacy"
      removed=1
    else
      echo "kept unrelated clutch binary at $legacy (version output did not identify it as Foam Clutch)" >&2
    fi
  done
fi

if command -v sluice >/dev/null 2>&1; then
  echo "warn: sluice still on PATH at $(command -v sluice) (another copy exists, use --all to scan all locations)" >&2
fi
[ "$removed" -eq 1 ] || { echo "nothing removed" >&2; exit 1; }
echo "uninstall done"
