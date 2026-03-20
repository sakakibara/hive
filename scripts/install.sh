#!/bin/sh
set -e

REPO="sakakibara/hive"
INSTALL_DIR="$HOME/.local/bin"

# Determine version: use argument or fetch latest.
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest release version." >&2
    exit 1
  fi
fi

# Strip leading 'v' for archive name (GoReleaser uses version without v prefix).
VERSION_NUM="${VERSION#v}"

# Detect OS and architecture.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin) ;;
  *) echo "Error: unsupported OS: $OS (only macOS is supported)" >&2; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64)        ARCH="amd64" ;;
  *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

ARCHIVE="hive_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading hive ${VERSION} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"

tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
mv "$TMPDIR/hive" "$INSTALL_DIR/hive"
chmod +x "$INSTALL_DIR/hive"

echo "Installed hive to $INSTALL_DIR/hive"
echo "Make sure $INSTALL_DIR is in your PATH."
