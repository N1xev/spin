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
#   curl -sSfL https://spin.pages.dev/install.sh | sh
#   curl -sSfL https://spin.pages.dev/install.sh | sh -s -- --force
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
      sed -n '2,9p' "$0"
      exit 0
      ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

# If Go is available and no specific version was requested, use go install.
if [[ -z "$VERSION" ]] && command -v go >/dev/null 2>&1; then
  echo "Installing $BIN via go install..."
  go install "github.com/$REPO@latest"
  echo "Installed to $(go env GOPATH)/bin/$BIN"
  echo "Run '$BIN help' to get started."
  exit 0
fi

# No Go available or specific version requested — download binary.
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

# Resolve the latest release tag from the GitHub API.
if [[ -z "$VERSION" ]]; then
  VERSION="$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/' || true)"
  if [[ -z "$VERSION" ]]; then
    echo "no release found; pass --version=vX.Y.Z or create a release first" >&2
    exit 1
  fi
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
echo "Run '$BIN help' to get started."
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo ""
  echo "Add $INSTALL_DIR to your PATH:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
