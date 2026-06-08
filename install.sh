#!/usr/bin/env sh
set -e

BASE_URL="${SFA_BASE_URL:-https://github.com/QuelThalasGrace/ssh-for-agents/releases/latest/download}"
INSTALL_DIR="$HOME/.local/bin"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) OS_NAME="darwin" ;;
  Linux) OS_NAME="linux" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) ARCH_NAME="arm64" ;;
  x86_64|amd64) ARCH_NAME="amd64" ;;
  *) echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

mkdir -p "$INSTALL_DIR"

URL="$BASE_URL/sfa-$OS_NAME-$ARCH_NAME"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$INSTALL_DIR/sfa"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$URL" -O "$INSTALL_DIR/sfa"
else
  echo "Missing curl or wget."
  exit 1
fi

chmod +x "$INSTALL_DIR/sfa"

echo "[ok] Installed sfa to $INSTALL_DIR/sfa"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo "[warn] $INSTALL_DIR is not in PATH."
    echo "Add this to your shell profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    ;;
esac

echo "Run: sfa doctor"
