#!/usr/bin/env bash
# Publish docs/wiki/*.md to the GitHub wiki repository.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE="${WIKI_REMOTE:-git@github.com:Inboxes-CIG/Bastion.wiki.git}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

git clone -q "$REMOTE" "$TMP"
find "$TMP" -maxdepth 1 -name '*.md' -delete
cp "$ROOT"/*.md "$TMP"/

cd "$TMP"
git add -A
if git diff --cached --quiet; then
  echo "Wiki is already up to date."
  exit 0
fi
git commit -q -m "Sync wiki from docs/wiki"
git push origin HEAD
echo "Wiki updated."
