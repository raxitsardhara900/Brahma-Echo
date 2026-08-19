#!/bin/bash
# Build release-style PinchTab binaries into ./dist for the current or requested platform.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION_VALUE="${VERSION:-dev}"
DIST_DIR="${DIST_DIR:-dist}"
MODE="${1:-current}"

build_one() {
  local goos_value="$1"
  local goarch_value="$2"
  local ext=""

  if [ "$goos_value" = "windows" ]; then
    ext=".exe"
  fi

  local output_path="$DIST_DIR/pinchtab-$goos_value-$goarch_value$ext"

  echo "📦 Building release-style binary..."
  echo "   target: ${goos_value}/${goarch_value}"
  echo "   output: ${output_path}"

  GOCACHE="${GOCACHE:-$PWD/.gocache}" \
  GOOS="$goos_value" \
  GOARCH="$goarch_value" \
  go build \
    -buildvcs=false \
    -ldflags="-s -w -X main.version=${VERSION_VALUE}" \
    -o "$output_path" \
    ./cmd/pinchtab

  if command -v stat >/dev/null 2>&1; then
    if stat -f '%z bytes %N' "$output_path" >/dev/null 2>&1; then
      stat -f '%z bytes %N' "$output_path"
    else
      stat -c '%s bytes %n' "$output_path"
    fi
  fi

  echo "✅ Binary complete: ${output_path}"
}

mkdir -p "$DIST_DIR" .gocache

./scripts/build-dashboard.sh

case "$MODE" in
  all)
    for goos_value in linux darwin windows; do
      for goarch_value in amd64 arm64; do
        build_one "$goos_value" "$goarch_value"
      done
    done
    ;;
  current)
    build_one "${GOOS:-$(go env GOOS)}" "${GOARCH:-$(go env GOARCH)}"
    ;;
  *)
    echo "usage: bash scripts/binary.sh [all]"
    exit 1
    ;;
esac

if [ "$MODE" = "all" ]; then
  echo "✅ All release binaries complete in ${DIST_DIR}/"
fi
