#!/bin/sh
# Post-install script for the rpm and deb packages.
#
# rpm: $1 is the number of installed instances (1 = install, 2+ = upgrade)
# deb: $1 is the action ("configure" or "abort-*"), $2 is the old version on
#      upgrade
set -e

upgrade=false
case "$1" in
    configure)
        if [ -n "$2" ]; then
            upgrade=true
        fi
        ;;
    [0-9]*)
        if [ "$1" -ge 2 ]; then
            upgrade=true
        fi
        ;;
    *)
        exit 0
        ;;
esac

if command -v systemd-sysusers >/dev/null 2>&1; then
    systemd-sysusers /usr/lib/sysusers.d/nflogfiled.conf
fi

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
    if [ "$upgrade" = true ]; then
        systemctl try-restart nflogfiled.service || true
    fi
fi
