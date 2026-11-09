#!/usr/bin/env sh
# Builds the Windows installer: build/bin/zebiventor-amd64-installer.exe
# It installs the till app and the headless hub (zebihub.exe), adds
# firewall rules for the local network, and keeps store data on uninstall.
#
# Needs: Go, Node, Wails CLI, and NSIS (makensis): `brew install makensis`
# on macOS, `apt install nsis` on Linux, or NSIS from nsis.sourceforge.io.
set -eu
cd "$(dirname "$0")/.."

VERSION=$(sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
LDFLAGS="-X github.com/onumahkalusamuel/zebiventor/internal/appdata.Version=$VERSION"
echo "Building Zebiventor $VERSION for Windows (amd64)"

mkdir -p build/bin
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS -s -w" -o build/bin/zebihub.exe ./cmd/zebihub
wails build -platform windows/amd64 -nsis -trimpath -ldflags "$LDFLAGS" "$@"

ls -lh build/bin/*.exe
