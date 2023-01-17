#!/bin/bash

set -euo pipefail

version="v0.0.1"
arch="amd64" # FIXME

err() {
    echo "$@" >&2
}

main() {

    # setup tempdir
    tmpdir="$(mktemp -d)"
    cd "$tmpdir"
    cleanup() {
        [ -z "$tmpdir" ] || rm -rf "$tmpdir"
    }
    trap cleanup EXIT

    # detect sudo
    if [ "$(id -u)" -eq 0 ]; then
        sudo=''
    elif command -v sudo >/dev/null; then
        sudo=sudo
    elif command -v doas >/dev/null; then
        sudo=doas
    else
        err 'You must be root'
        exit 1
    fi

    # download tar
    curl -o rvpn.tar.gz "https://github.com/redpwn/rvpn/releases/download/$version/rvpn_linux_$arch.tar.gz"

    # extract tar
    tar -xzvf rvpn.tar.gz

    # install rvpn and rvpn service
    $sudo install rvpn_linux_$arch/systemd/rvpn.service /etc/systemd/system/
    $sudo install rvpn_linux_$arch/bin/rvpn /usr/local/bin
}

main