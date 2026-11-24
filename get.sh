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
sudo ./install.sh
