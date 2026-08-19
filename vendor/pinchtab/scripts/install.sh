#!/bin/bash
# Build Pinchtab and install it locally for development use.
# Builds straight into the install path so the linker's ad-hoc signature
# stays valid on Apple Silicon (a plain `cp` invalidates it and macOS
# kills the process on launch).
set -euo pipefail

cd "$(dirname "$0")/.."

INSTALL_DIR="${PINCHTAB_INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_PATH="$INSTALL_DIR/pinchtab"

mkdir -p "$INSTALL_DIR"

./scripts/build-dashboard.sh

echo "🔨 Building Go → $INSTALL_PATH"
go build -o "$INSTALL_PATH" ./cmd/pinchtab

if [[ "$(uname -s)" == "Darwin" ]]; then
	echo "🔏 Re-signing (ad-hoc) for macOS"
	codesign --force --sign - "$INSTALL_PATH"
fi

if command -v brew >/dev/null 2>&1 && brew list pinchtab >/dev/null 2>&1; then
	if [[ -L /opt/homebrew/bin/pinchtab || -L /usr/local/bin/pinchtab ]]; then
		echo "⚠️  Homebrew pinchtab is linked and may shadow this build."
		echo "    Run: brew unlink pinchtab"
	fi
fi

echo "✅ Installed: $INSTALL_PATH"
"$INSTALL_PATH" --version
