#!/usr/bin/env bash
set -e

BINARY="obelisk"
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="$HOME/.config/obelisk"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[obelisk]${NC} $*"; }
warn()  { echo -e "${YELLOW}[obelisk]${NC} $*"; }

KEEP_IDENTITY=true
for arg in "$@"; do
  [[ "$arg" == "--all" ]] && KEEP_IDENTITY=false
done

if [ -f "$INSTALL_DIR/$BINARY" ]; then
  rm "$INSTALL_DIR/$BINARY"
  info "Removed $INSTALL_DIR/$BINARY"
else
  warn "Binary not found at $INSTALL_DIR/$BINARY — skipping"
fi

if [ -d "$CONFIG_DIR" ]; then
  if $KEEP_IDENTITY; then
    # Remove registry but keep keypair so re-install doesn't change identity
    if [ -f "$CONFIG_DIR/servers.yml" ]; then
      rm "$CONFIG_DIR/servers.yml"
      info "Removed $CONFIG_DIR/servers.yml"
    fi
    warn "Kept identity keys in $CONFIG_DIR (use --all to remove everything)"
  else
    rm -rf "$CONFIG_DIR"
    info "Removed $CONFIG_DIR"
  fi
fi

info "Uninstall complete."
