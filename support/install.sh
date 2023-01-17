#!/bin/bash

set -euo pipefail

version="v0.0.1"
arch="amd64" # FIXME

enable_ip4_forwarding() {
    # arg $1 is the sudo string
    echo "net.ipv4.ip_forward = 1" | "$1" tee -a /etc/sysctl.conf
    $1 sysctl -p /etc/sysctl.conf
}

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
    curl -sSLo rvpn.tar.gz "https://github.com/redpwn/rvpn/releases/download/$version/rvpn_linux_$arch.tar.gz"

    # extract tar
    tar -xzvf rvpn.tar.gz

    # ask user if they want to enable ip forwarding if not already enabled
    if [ "$(cat /proc/sys/net/ipv4/ip_forward)" -eq 0 ]; then
        echo "Do you wish to enable IP Forwarding (only used if this device is a VPN server)? [1,2]"
        select yn in "Yes" "No"; do
            case $yn in
                Yes ) enable_ip4_forwarding $sudo ; break;;
                No ) break;;
            esac
        done
    fi

    # install rvpn and rvpn service
    $sudo install -Dm 644 -t /usr/local/lib/systemd/system/ rvpn_linux_$arch/systemd/systemd/rvpn.service
    $sudo install -m 755 -t /usr/local/bin/ rvpn_linux_$arch/bin/rvpn

    # print success and remind user to allow rvpn serve port (21820) through firewall if serving
    echo "Successfully installed rVPN!"
    echo "If using this device as a VPN server please allow rVPN serve port (default 21820) through firewall"
}

main