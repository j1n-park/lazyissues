#!/usr/bin/env sh
set -eu

APP_NAME="lazyissues"
CMD_PATH="./cmd/lazyissues"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"

usage() {
  cat <<EOF
Install $APP_NAME.

Usage: ./install.sh [--prefix DIR] [--bindir DIR]

Environment:
  PREFIX   Install prefix. Defaults to \$HOME/.local.
  BINDIR   Directory for the executable. Defaults to \$PREFIX/bin.

Examples:
  ./install.sh
  PREFIX=/usr/local ./install.sh
  BINDIR=/tmp/bin ./install.sh
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --prefix)
      [ "$#" -ge 2 ] || { echo "error: --prefix requires a directory" >&2; exit 2; }
      PREFIX="$2"
      BINDIR="$PREFIX/bin"
      shift 2
      ;;
    --prefix=*)
      PREFIX=${1#--prefix=}
      BINDIR="$PREFIX/bin"
      shift
      ;;
    --bindir)
      [ "$#" -ge 2 ] || { echo "error: --bindir requires a directory" >&2; exit 2; }
      BINDIR="$2"
      shift 2
      ;;
    --bindir=*)
      BINDIR=${1#--bindir=}
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "error: Go is required to build $APP_NAME" >&2
  exit 1
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

printf 'Building %s...\n' "$APP_NAME"
go build -o "$tmpdir/$APP_NAME" "$CMD_PATH"

printf 'Installing %s to %s...\n' "$APP_NAME" "$BINDIR"
mkdir -p "$BINDIR"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$tmpdir/$APP_NAME" "$BINDIR/$APP_NAME"
else
  cp "$tmpdir/$APP_NAME" "$BINDIR/$APP_NAME"
  chmod 0755 "$BINDIR/$APP_NAME"
fi

case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *)
    printf 'Note: %s is not currently on PATH. Add it to run %s from anywhere.\n' "$BINDIR" "$APP_NAME" >&2
    ;;
esac

printf 'Installed %s\n' "$BINDIR/$APP_NAME"
