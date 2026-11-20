#!/bin/sh
# Post-remove script for the rpm and deb packages.
#
# This runs on both removal and upgrade. The service user and group are left
# in place, following common distro practice for system users.
set -e

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi
