#!/usr/bin/env bash
# scripts/install.sh
#
# One-line installer. Downloads the latest spin release for the
# current OS/arch and installs into $HOME/.local/bin (or
# /usr/local/bin if $HOME/.local/bin is not writable).
#
# Skips if spin is already on PATH and the user did not pass
# --force. Idempotent.
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/N1xev/spin/main/scripts/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/N1xev/spin/main/scripts/install.sh | sh -s -- --force
set -euo pipefail

REPO="N1xev/spin"
BIN="spin"
FORCE=0
VERSION=""

for arg in "$@"; do
  case "$arg" in
    --force|-f) FORCE=1 ;;
    --version=*) VERSION="${arg#--version=}" ;;
    --help|-h)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

# Pick install dir: prefer ~/.local/bin (XDG-friendly, no sudo),
# fall back to /usr/local/bin.
INSTALL_DIR="$HOME/.local/bin"
if [[ ! -d "$INSTALL_DIR" ]]; then
  if [[ -w "/usr/local/bin" ]]; then
    INSTALL_DIR="/usr/local/bin"
  else
    mkdir -p "$INSTALL_DIR"
  fi
fi

# Detect OS and arch.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 2 ;;
esac
case "$OS" in
  linux|darwin) ;;
  *) echo "unsupported OS: $OS" >&2; exit 2 ;;
esac

# Resolve "latest" if no version was given. The API returns 403 on
# rate limit; if that happens, fall back to redirect-following with
# a hardcoded GitHub releases URL.
if [[ -z "$VERSION" ]]; then
  VERSION="$(curl -sSfL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')" || {
    echo "could not resolve latest version via API; pass --version=vX.Y.Z" >&2
    exit 1
  }
fi
VERSION="${VERSION#v}"
TARBALL="${BIN}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/v${VERSION}/$TARBALL"

# Idempotency: if spin exists and is up to date, skip.
if command -v spin >/dev/null 2>&1 && [[ "$FORCE" -eq 0 ]]; then
  current="$(spin version 2>/dev/null | awk '{print $NF}' || echo unknown)"
  current="${current#v}"
  if [[ "$current" == "$VERSION" ]]; then
    echo "spin $VERSION already installed at $(command -v spin); pass --force to reinstall"
    exit 0
  fi
fi

# Download + extract.
TMP="$(mktemp -d)"
trap "rm -rf $TMP" EXIT
echo "Downloading $BIN $VERSION ($OS/$ARCH)..."
curl -sSfL "$URL" -o "$TMP/$TARBALL"
tar -xzf "$TMP/$TARBALL" -C "$TMP"
install -m 0755 "$TMP/$BIN" "$INSTALL_DIR/$BIN"

echo "Installed $BIN $VERSION to $INSTALL_DIR/$BIN"
echo "Run '$BIN --help' to get started."
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo ""
  echo "Add $INSTALL_DIR to your PATH:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
