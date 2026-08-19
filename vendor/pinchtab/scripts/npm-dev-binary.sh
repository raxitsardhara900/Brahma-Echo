#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_PATH="${1:-$ROOT_DIR/pinchtab-dev}"

echo "Building local PinchTab binary at $OUT_PATH"
(cd "$ROOT_DIR" && go build -o "$OUT_PATH" ./cmd/pinchtab)

cat <<EOF

Built local binary:
  $OUT_PATH

Canonical local npm path:
  $ROOT_DIR/pinchtab-dev

Use it with the npm package:
  cd "$ROOT_DIR/npm"
  npm install
  node bin/pinchtab --version
EOF
