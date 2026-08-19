#!/usr/bin/env bash
# Produce a ClawHub-ready tarball of plugin/ for manual upload at
# https://clawhub.ai/publish-plugin.
#
# Usage:
#   scripts/pack-plugin.sh                # pack at the version in package.json
#   scripts/pack-plugin.sh 0.11.1         # bump to 0.11.1, pack, restore the
#                                         # working-tree version (so the bump
#                                         # is not committed by mistake)
#
# Output goes to dist/plugin-pack/<name>-<version>.tgz and the path is echoed
# at the end so you can drop it straight into the ClawHub upload form.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-}"
PLUGIN_DIR="$(pwd)/plugin"
OUT_DIR="$(pwd)/dist/plugin-pack"
mkdir -p "$OUT_DIR"

if [[ ! -d "$PLUGIN_DIR" ]]; then
  echo "no plugin directory at $PLUGIN_DIR" >&2
  exit 1
fi

ORIGINAL_PKG=$(cat "$PLUGIN_DIR/package.json")
ORIGINAL_MANIFEST=$(cat "$PLUGIN_DIR/openclaw.plugin.json")

restore() {
  printf '%s' "$ORIGINAL_PKG" > "$PLUGIN_DIR/package.json"
  printf '%s' "$ORIGINAL_MANIFEST" > "$PLUGIN_DIR/openclaw.plugin.json"
}
trap restore EXIT

if [[ -n "$VERSION" ]]; then
  echo "→ bumping plugin version to $VERSION (working-tree only, restored on exit)"
  (cd "$PLUGIN_DIR" && npm version "$VERSION" --no-git-tag-version >/dev/null)
  node -e "
    const fs = require('fs');
    const path = '$PLUGIN_DIR/openclaw.plugin.json';
    const m = JSON.parse(fs.readFileSync(path, 'utf8'));
    m.version = '$VERSION';
    fs.writeFileSync(path, JSON.stringify(m, null, 2) + '\n');
  "
fi

echo "→ installing plugin deps (needed for tsc in prepack)"
(cd "$PLUGIN_DIR" && npm install --ignore-scripts --no-package-lock --no-audit --no-fund >/dev/null)

echo "→ packing via clawhub (mirrors what reusable-publish-plugin.yml uploads)"
npx -y -p clawhub clawhub package pack "$PLUGIN_DIR" --pack-destination "$OUT_DIR"

TARBALL=$(ls -t "$OUT_DIR"/*.tgz 2>/dev/null | head -1 || true)
if [[ -z "$TARBALL" ]]; then
  echo "no tarball produced — check the output above" >&2
  exit 1
fi

echo
echo "ready: $TARBALL"
echo "       $(wc -c <"$TARBALL" | tr -d ' ') bytes"
echo "       sha256 $(shasum -a 256 "$TARBALL" | awk '{print $1}')"
echo
echo "upload at https://clawhub.ai/publish-plugin"
