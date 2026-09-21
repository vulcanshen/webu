#!/bin/sh
# webu installer for macOS / Linux (unix-only — on Windows use WSL).
# Usage: curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh

set -e

REPO="vulcanshen/webu"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *) echo "Error: webu is unix-only (no Windows build — use WSL). Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# webu runs only the pinned Chromium snapshot, and the snapshot bucket has
# no Linux ARM build.
if [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
  echo "Error: no Chromium snapshot for linux/arm64, so there is no webu build for it."
  exit 1
fi

# Get latest version
echo "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"v\(.*\)".*/\1/')
echo "Latest version: $VERSION"

# Install dir
if [ "$(id -u)" = "0" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

FILENAME="webu_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v${VERSION}/$FILENAME"

# Download
TMPDIR=$(mktemp -d)
echo "Downloading $FILENAME..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/$FILENAME"

# Extract
echo "Extracting..."
tar xzf "$TMPDIR/$FILENAME" -C "$TMPDIR"

# Install
mkdir -p "$INSTALL_DIR"
cp "$TMPDIR/webu" "$INSTALL_DIR/webu"
chmod +x "$INSTALL_DIR/webu"
rm -rf "$TMPDIR"

echo ""
echo "webu $VERSION installed to $INSTALL_DIR"

# Check if install dir is in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "WARNING: $INSTALL_DIR is not in your PATH. Add it by running:"
    echo ""
    echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.$(basename "$SHELL")rc && source ~/.$(basename "$SHELL")rc"
    ;;
esac

echo ""
echo "NOTE: webu runs its own Chromium (a pinned revision). The first launch"
echo "      downloads it once — about 175–250 MB — into the cache directory"
echo "      (macOS: ~/Library/Caches/webu, Linux: ~/.cache/webu) and says so."
echo ""
echo "NOTE: webu draws links, media, panels and the header with Nerd Font glyphs."
echo "      Use a Nerd Font terminal profile (https://www.nerdfonts.com) or those"
echo "      cells will render as boxes."
echo ""
echo "Run 'webu' (or 'webu https://example.com') to launch."
