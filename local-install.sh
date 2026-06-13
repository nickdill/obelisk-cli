#!/usr/bin/env bash
set -e

BINARY="obelisk"
INSTALL_DIR="$HOME/.local/bin"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[obelisk]${NC} $*"; }
error() { echo -e "${RED}[obelisk]${NC} $*" >&2; exit 1; }

info "Building..."
go build -ldflags "-X cmd.version=dev" -o "$BINARY" . || error "Build failed"

mkdir -p "$INSTALL_DIR"
cp "$BINARY" "$INSTALL_DIR/$BINARY"
rm "$BINARY"

info "Installed dev build to $INSTALL_DIR/$BINARY"
