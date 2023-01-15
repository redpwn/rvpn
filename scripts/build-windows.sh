#!/bin/bash
set -euo pipefail

# switch directory
cd "$(dirname "$(dirname "$0")")"
cd "../"

# initial wails build
cd "./cmd/client_gui"
wails build

# build cli client
cd "../../"
go build -o "./cmd/client_gui/build/bin/rvpnc.exe" "./cmd/client"

# copy vendored wintun to wails build dir
cp "./vendored/wintun.dll" "./cmd/client_gui/build/bin/"

# wails build nsis installer
cd "./cmd/client_gui"
wails build -nsis
