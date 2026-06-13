#!/usr/bin/env bash
set -e

REPO="nickdill/obelisk-cli"
BINARY="obelisk"
INSTALL_DIR="$HOME/.local/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[obelisk]${NC} $*"; }
warn()  { echo -e "${YELLOW}[obelisk]${NC} $*"; }
error() { echo -e "${RED}[obelisk]${NC} $*" >&2; exit 1; }

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *) error "Unsupported OS: $OS. Only macOS and Linux are supported." ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH." ;;
esac

if [ -n "$OBELISK_VERSION" ]; then
  VERSION="$OBELISK_VERSION"
  info "Using version $VERSION (from OBELISK_VERSION)"
else
  info "Fetching latest release..."
  API_RESPONSE=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null || true)

  if echo "$API_RESPONSE" | grep -q '"Not Found"'; then
    error "No releases found for $REPO. Push a git tag to publish the first release:
  git tag v0.1.0 && git push origin v0.1.0"
  fi

  VERSION=$(echo "$API_RESPONSE" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

  if [ -z "$VERSION" ]; then
    error "Could not determine latest version. Check your internet connection and try again."
  fi
fi

ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"

mkdir -p "$INSTALL_DIR"
info "Downloading $BINARY $VERSION ($OS/$ARCH)..."
curl -fsSL "$URL" -o "$INSTALL_DIR/$BINARY" || error "Download failed: $URL"
chmod +x "$INSTALL_DIR/$BINARY"

ensure_path() {
  local file="$1"
  [ -f "$file" ] || return 0
  grep -q "$INSTALL_DIR" "$file" 2>/dev/null && return 0
  printf '\nexport PATH="$PATH:%s"\n' "$INSTALL_DIR" >> "$file"
  echo "$file"
}

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  warn "$INSTALL_DIR is not in your PATH — adding it now..."
  modified=()
  for rc in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.profile"; do
    result=$(ensure_path "$rc")
    [ -n "$result" ] && modified+=("$result")
  done

  if [ ${#modified[@]} -gt 0 ]; then
    warn "Updated: ${modified[*]}"
    warn "Run one of the following to use obelisk immediately:"
    for f in "${modified[@]}"; do
      warn "  source $f"
    done
    warn "Or open a new terminal."
  fi
else
  info "$INSTALL_DIR is already in your PATH."
fi

info "$BINARY $VERSION installed to $INSTALL_DIR/$BINARY"
info "Run 'obelisk --help' to get started."
