#!/bin/sh
# webu installer for macOS / Linux.
# Usage: curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/install.sh | sh
#
# terminu family template (tdp project/install.sh). Replace webu; set
# WINDOWS; the blocks marked "app's own" are the app's, the rest stays as it is.

set -e

APP="webu"
REPO="vulcanshen/$APP"
# yes when the app ships a Windows build and an install.ps1.
WINDOWS="no"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *)
    echo "Error: unsupported OS: $OS."
    if [ "$WINDOWS" = "yes" ]; then
      echo "On Windows, run in PowerShell:"
      echo "  irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex"
    else
      echo "$APP is unix-only (no Windows build — use WSL)."
    fi
    exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# --- app's own: checks before the download (none by default) ---
# webu runs only the pinned Chromium snapshot, and the snapshot bucket has
# no Linux ARM build.
if [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
  echo "Error: no Chromium snapshot for linux/arm64, so there is no webu build for it."
  exit 1
fi
# --- end of the app's own ---

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

FILENAME="${APP}_${VERSION}_${OS}_${ARCH}.tar.gz"
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
cp "$TMPDIR/$APP" "$INSTALL_DIR/$APP"
chmod +x "$INSTALL_DIR/$APP"
rm -rf "$TMPDIR"

echo ""
echo "$APP $VERSION installed to $INSTALL_DIR"

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

# --- app's own: what to know, and what to run first ---
echo ""
echo "NOTE: $APP runs its own Chromium (a pinned revision). The first launch"
echo "      downloads it once — about 175–250 MB — into the cache directory"
echo "      (macOS: ~/Library/Caches/webu, Linux: ~/.cache/webu) and says so."
echo ""
echo "NOTE: $APP draws with Nerd Font glyphs. Use a Nerd Font terminal"
echo "      profile (https://www.nerdfonts.com) or they render as boxes."
echo "      It also needs a truecolor (24-bit) terminal; with 256 colours"
echo "      the dimmed and layered colours can't be told apart."
echo ""
echo "Run '$APP' (or '$APP https://example.com') to start."
# --- end of the app's own ---
