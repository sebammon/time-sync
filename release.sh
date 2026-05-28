#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "usage: ./release.sh vX.Y.Z" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must look like vX.Y.Z (got: $VERSION)" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is dirty — commit or stash first" >&2
  exit 1
fi

rm -rf dist
mkdir -p dist

echo "→ building darwin/arm64"
GOOS=darwin GOARCH=arm64 go build -o "dist/time-sync-darwin-arm64" .

echo "→ creating release $VERSION"
gh release create "$VERSION" dist/* --generate-notes
