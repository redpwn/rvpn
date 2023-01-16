#!/bin/bash
set -euo pipefail

# switch directory
cd "$(dirname "$(dirname "$0")")"
cd "../"

# create out dir
mkdir "dist"

# build cli binary
go build -o "./dist/rvpn" "./cmd/client/"

# create tar with files
tar -czvf "./dist/linux_cli.tar.gz" "./support/systemd/rvpn.service" "./dist/rvpn"