#!/bin/bash
# Build Pinchtab and the embedded dashboard without starting the server.
set -euo pipefail

cd "$(dirname "$0")/.."

./scripts/build-dashboard.sh

echo "🔨 Building Go..."
go build -o pinchtab ./cmd/pinchtab

echo "✅ Build complete: ./pinchtab"
