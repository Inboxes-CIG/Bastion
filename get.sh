#!/bin/sh
set -e
REPO="Inboxes-CIG/Bastion"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
URL="https://github.com/${REPO}/releases/latest/download/bastion_linux_${ARCH}.tar.gz"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
echo "Downloading Bastion for linux/${ARCH}..."
curl -fsSL "$URL" -o "$TMP/bastion.tar.gz"
tar -xzf "$TMP/bastion.tar.gz" -C "$TMP"
cd "$TMP"
chmod +x install.sh
# Already root (e.g. "curl | sudo bash"): run directly. A second sudo would
# create a new session/pty (sudo use_pty) where interactive prompts are
# stopped by SIGTTIN and the install appears to hang.
if [ "$(id -u)" -eq 0 ]; then
  exec ./install.sh "$@"
else
  exec sudo ./install.sh "$@"
fi
