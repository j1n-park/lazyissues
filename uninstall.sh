#!/usr/bin/env sh
set -eu

APP_NAME="lazyissues"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"

usage() {
  cat <<EOF
Uninstall $APP_NAME.

Usage: ./uninstall.sh [--prefix DIR] [--bindir DIR]

Environment:
  PREFIX   Install prefix. Defaults to \$HOME/.local.
  BINDIR   Directory containing the executable. Defaults to \$PREFIX/bin.

Examples:
  ./uninstall.sh
  PREFIX=/usr/local ./uninstall.sh
  BINDIR=/tmp/bin ./uninstall.sh
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

target="$BINDIR/$APP_NAME"

if [ ! -e "$target" ]; then
  printf '%s is not installed at %s\n' "$APP_NAME" "$target"
  exit 0
fi

if [ -d "$target" ]; then
  echo "error: refusing to remove directory: $target" >&2
  exit 1
fi

rm -f "$target"
printf 'Removed %s\n' "$target"
