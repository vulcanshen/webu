#!/bin/sh
# webu uninstaller for macOS / Linux.
# Usage: curl -fsSL https://raw.githubusercontent.com/vulcanshen/webu/main/uninstall.sh | sh

set -e

CANDIDATES="$HOME/.local/bin/webu /usr/local/bin/webu"

FOUND=""
for path in $CANDIDATES; do
  if [ -f "$path" ]; then
    FOUND="$path"
    break
  fi
done

if [ -z "$FOUND" ]; then
  echo "webu not found in expected locations."
  echo "Checked: $CANDIDATES"
  exit 1
fi

rm "$FOUND"
echo "removed $FOUND"

# ask offers to remove one directory: the answer is read from the terminal,
# not stdin — under `curl ... | sh` stdin is the script itself, so a plain
# `read` would consume script text (or hit EOF) instead of the keypress. No
# controlling terminal (cron, nohup) -> keep it, the safe default.
ask() {
  dir="$1"; what="$2"
  [ -d "$dir" ] || return 0
  printf "Remove %s in %s? [y/N]: " "$what" "$dir"
  if read -r answer < /dev/tty 2>/dev/null; then
    case "$answer" in
      y|Y|yes|YES) rm -rf "$dir"; echo "removed $dir" ;;
      *) echo "kept $dir" ;;
    esac
  else
    echo ""
    echo "kept $dir (no terminal to confirm on)"
  fi
}

# What the user wrote: settings and bookmarks. XDG_CONFIG_HOME wins when set,
# else ~/.config/webu on every platform (matching the binary).
if [ -n "$XDG_CONFIG_HOME" ]; then
  CONFIG_DIR="$XDG_CONFIG_HOME/webu"
else
  CONFIG_DIR="$HOME/.config/webu"
fi
ask "$CONFIG_DIR" "webu settings and bookmarks"

# What webu produced: history, session, downloads, the browser profile
# (cookies, logins) and its log.
ask "$HOME/.webu/datas" "webu history, session, downloads and browser profile"

# The pinned Chromium itself: re-downloadable, and the biggest of the three.
if [ -n "$XDG_CACHE_HOME" ]; then
  CACHE_DIR="$XDG_CACHE_HOME/webu"
elif [ "$(uname -s)" = "Darwin" ]; then
  CACHE_DIR="$HOME/Library/Caches/webu"
else
  CACHE_DIR="$HOME/.cache/webu"
fi
ask "$CACHE_DIR" "the downloaded Chromium (re-fetched on the next launch)"

echo ""
echo "webu uninstalled."
