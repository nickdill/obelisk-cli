#!/usr/bin/env bash
set -e

BINARY="obelisk"
INSTALL_DIR="$HOME/.local/bin"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[obelisk]${NC} $*"; }
error() { echo -e "${RED}[obelisk]${NC} $*" >&2; exit 1; }

# Find go — check PATH first, then the standard install location
GO=$(command -v go 2>/dev/null || echo "/usr/local/go/bin/go")
[ -x "$GO" ] || error "go not found — install from https://go.dev/dl/"

info "Building..."
"$GO" build -ldflags "-X cmd.version=dev" -o "$BINARY" . || error "Build failed"

mkdir -p "$INSTALL_DIR"
cp "$BINARY" "$INSTALL_DIR/$BINARY"
rm "$BINARY"

info "Installed dev build to $INSTALL_DIR/$BINARY"

# Warn if install dir isn't in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) info "Note: add $INSTALL_DIR to your PATH to use 'obelisk' directly" ;;
esac
