#!/bin/bash
# Bastion build script
set -e
SRC="$(cd "$(dirname "$0")" && pwd)"
VERSION="${VERSION:-$(cat "$SRC/VERSION" 2>/dev/null || echo dev)}"
cd "$SRC"
echo "=== Building frontend (v$VERSION) ==="
( cd web && npm run build )
echo "=== Embedding frontend ==="
rm -rf internal/web/dist
cp -r web/dist internal/web/dist
echo "=== Building Go binary ==="
CGO_ENABLED=0 go build -ldflags "-X github.com/inboxes-cig/bastion/internal/version.Version=$VERSION" -o /opt/bastion/bin/bastion ./cmd/bastion
echo "=== Done: $(ls -lh /opt/bastion/bin/bastion | awk '{print $5}') ==="
