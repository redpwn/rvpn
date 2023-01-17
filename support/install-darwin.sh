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
    curl -sSLo rvpn.tar.gz "https://github.com/redpwn/rvpn/releases/download/$version/rvpn_darwin_$arch.tar.gz"

    # extract tar
    tar -xzvf rvpn.tar.gz

    # install rvpn and rvpn service
    $sudo install -Dm 644 -t /Library/LaunchDaemons rvpn_darwin_$arch/launchd/rvpn.service
    $sudo install -m 755 -t /usr/local/bin/ rvpn_linux_$arch/bin/rvpn

    $sudo launchctl load /Library/LaunchDaemons/dev.rvpn.plist

    echo "Start rvpn daemon?"
    select yn in "Start" "Skip"; do
        case $yn in
            "Start" ) $sudo launchctl start /Library/LaunchDaemons/dev.rvpn.plist break;;
            "Skip" ) break;;
        esac
    done

    # print success and remind user to allow rvpn serve port (21820) through firewall if serving
    echo "Successfully installed rVPN!"
}

main