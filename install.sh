#!/usr/bin/env bash
# install.sh - build and install the `sluice` CLI.
# Usage:
#   ./install.sh [--prefix ~/.local] [--bindir <dir>] [--system] [--version <ver>] [-h|--help]
#
# Defaults: PREFIX="$HOME/.local", BINDIR="$PREFIX/bin" (i.e. ~/.local/bin).
#   --system  == --bindir /usr/local/bin (needs sudo).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR=""
VERSION=""
SYSTEM=0

usage() {
  sed -n '2,10p' "$0"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --prefix) PREFIX="${2:?--prefix needs a value}"; shift 2 ;;
    --bindir) BINDIR="${2:?--bindir needs a value}"; shift 2 ;;
    --system) SYSTEM=1; shift ;;
    --version) VERSION="${2:?--version needs a value}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$SYSTEM" -eq 1 ]; then
  BINDIR="/usr/local/bin"
elif [ -z "$BINDIR" ]; then
  BINDIR="$PREFIX/bin"
fi

if ! command -v go >/dev/null 2>&1; then
  echo "error: go not found on PATH (need Go 1.26+)" >&2
  exit 1
fi

# Need Go >= 1.26 (README requirement).
GOVER="$(go version | awk '{print $3}' | sed 's/^go//')"
if ! printf '%s\n%s\n' "1.26.0" "$GOVER" | sort -V -C; then
  echo "error: go $GOVER < 1.26.0, upgrade first" >&2
  go version >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  if command -v git >/dev/null 2>&1 && git -C "$REPO_ROOT" rev-parse --short HEAD >/dev/null 2>&1; then
    VERSION="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
  else
    VERSION="dev"
  fi
fi

echo "building sluice version=$VERSION"
cd "$REPO_ROOT"
TMP_BIN="$(mktemp -d)/sluice"
trap 'rm -rf "$(dirname "$TMP_BIN")"' EXIT
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.Version=${VERSION}" \
  -o "$TMP_BIN" ./cmd/sluice

mkdir -p "$BINDIR"
if [ -w "$BINDIR" ]; then
  install -m 0755 "$TMP_BIN" "$BINDIR/sluice"
else
  echo "need write access to $BINDIR, retrying with sudo"
  sudo install -m 0755 "$TMP_BIN" "$BINDIR/sluice"
fi

"$BINDIR/sluice" version
echo "installed to $BINDIR/sluice"

# Legacy notice: never delete the old copy, only inform.
if command -v clutch >/dev/null 2>&1; then
  OLD_BIN="$(command -v clutch)"
  OLD_OUT="$("$OLD_BIN" version 2>&1 || true)"
  case "$OLD_OUT" in
    *"Foam Clutch"*|*"foam-clutch"*|clutch\ *)
      echo "notice: legacy Foam Clutch install found at $OLD_BIN; it was left in place (remove it manually after migrating; see README Migrating from Foam Clutch)" >&2
      ;;
    *)
      echo "notice: another binary named clutch found at $OLD_BIN; it was left in place" >&2
      ;;
  esac
fi

# PATH hint.
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "note: $BINDIR not on PATH. Add: export PATH=\"$BINDIR:\$PATH\"" >&2 ;;
esac

# Optional runtime deps: warn only, install still succeeds.
for cmd in sbatch mpirun; do
  command -v "$cmd" >/dev/null 2>&1 || echo "warn: $cmd not on PATH (needed at submit/run time)" >&2
done
if [ -z "${FOAM_BASHRC:-}" ] && [ ! -f /opt/openfoam12/etc/bashrc ]; then
  echo "warn: no OpenFOAM bashrc found (set FOAM_BASHRC or use runtime.foamBashrc)" >&2
fi
