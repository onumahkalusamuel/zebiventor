#!/usr/bin/env sh
# Builds and signs a release: ./scripts/release.sh "What changed"
# Needs ZEBI_UPDATE_KEY (public key) and release.key (private, from
# `go run ./cmd/zebirelease keygen`). Output in dist/: publish manifest.json,
# manifest.json.sig and the installer at the update feed address, and/or hand
# out zebiventor-<version>-update.zip to load into a hub by hand.
set -eu
cd "$(dirname "$0")/.."
: "${ZEBI_UPDATE_KEY:?set ZEBI_UPDATE_KEY to the public key from zebirelease keygen}"
VERSION=$(sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
./scripts/build-windows.sh
go run ./cmd/zebirelease sign -key "${ZEBI_RELEASE_KEY:-release.key}" -version "$VERSION" -notes "${1:-}" -out dist \
  build/bin/zebiventor-amd64-installer.exe
