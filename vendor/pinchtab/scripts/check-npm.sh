#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR/npm"

echo "Verifying npm package..."

echo "Building source-checkout npm binary..."
(cd "$ROOT_DIR" && go build -o pinchtab-dev ./cmd/pinchtab)

DESIRED_VERSION="${1:-}"
if [ -n "$DESIRED_VERSION" ]; then
  CURRENT_VERSION="$(jq -r .version package.json)"
  if [ "$CURRENT_VERSION" != "$DESIRED_VERSION" ]; then
    npm version "$DESIRED_VERSION" --no-git-tag-version
  else
    echo "Version already correct: $CURRENT_VERSION"
  fi
fi

npm ci
npm run lint
npm run format:check
npx tsc
npm test

npm pack >/dev/null 2>&1
TARBALL="$(ls -t pinchtab-*.tgz | head -1)"
trap 'rm -f "$TARBALL"' EXIT

SOURCES="$(tar -tzf "$TARBALL" | grep -E '\.ts$' | grep -v '\.d\.ts' || true)"
if [ -n "$SOURCES" ]; then
  echo "ERROR: Source .ts files in package:"
  echo "$SOURCES"
  exit 1
fi

TESTS="$(npm pack --dry-run 2>&1 | grep 'dist/tests' || true)"
if [ -n "$TESTS" ]; then
  echo "ERROR: Test files in package:"
  echo "$TESTS"
  exit 1
fi

MAPS="$(npm pack --dry-run 2>&1 | grep '\.map$' || true)"
if [ -n "$MAPS" ]; then
  echo "ERROR: Source maps in package:"
  echo "$MAPS"
  exit 1
fi

REQUIRED=(
  "dist/src/index.js"
  "dist/src/index.d.ts"
  "scripts/postinstall.js"
  "scripts/sync-skills.js"
  "skills/pinchtab/SKILL.md"
  "bin/pinchtab"
  "LICENSE"
)

PACK_OUTPUT="$(npm pack --dry-run 2>&1)"
for file in "${REQUIRED[@]}"; do
  if ! echo "$PACK_OUTPUT" | grep -q "$file"; then
    echo "ERROR: Missing required package file: $file"
    exit 1
  fi
done

node -c scripts/postinstall.js
node -c scripts/stage-skills.js
node -c scripts/sync-skills.js

node -e "
  const pkg = require('./package.json');
  const checks = [
    { name: 'name', value: pkg.name === 'pinchtab' },
    { name: 'version', value: !!pkg.version },
    { name: 'files array', value: Array.isArray(pkg.files) && pkg.files.length > 0 },
    { name: 'postinstall script', value: !!pkg.scripts.postinstall },
    { name: 'bin.pinchtab', value: !!pkg.bin.pinchtab }
  ];

  let failed = false;
  for (const check of checks) {
    if (!check.value) {
      console.error('ERROR:', check.name, 'missing or invalid');
      failed = true;
    }
  }

  if (failed) process.exit(1);
"

npm audit --audit-level=moderate || true

echo "npm package verification passed."
