#!/bin/bash
set -euo pipefail

# switch directory to repo root
cd "$(dirname "$0")/.."

# build cli client
go build -o "./cmd/client_gui/build/bin/rvpnc.exe" "./cmd/client"

# copy vendored wintun to wails build dir
cp "./vendored/wintun.dll" "./cmd/client_gui/build/bin/"

# wails build nsis installer
(cd "./cmd/client_gui" && wails build -platform "windows/amd64" -nsis)
