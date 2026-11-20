#!/bin/sh
# Pre-remove script for the rpm and deb packages.
#
# rpm: $1 is the number of instances left after removal (0 = removal)
# deb: $1 is the action ("remove", "upgrade", "deconfigure", ...)
set -e

case "$1" in
    0|remove) ;;
    *) exit 0 ;;
esac

if [ -d /run/systemd/system ]; then
    systemctl disable --now nflogfiled.service || true
elif command -v systemctl >/dev/null 2>&1; then
    # Without a running systemd, only remove the enablement symlinks
    systemctl disable nflogfiled.service || true
fi
