#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

cd "$ROOT"
make build INSTALL_DIR="$INSTALL_DIR"

echo ""
echo "Installed: $INSTALL_DIR/tsll"
echo ""
echo "One-time setup (no root) — add to PATH:"
echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc"
echo "  source ~/.bashrc"
echo ""
echo "Then run: tsll"
