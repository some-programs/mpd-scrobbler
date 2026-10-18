#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/mpd-scrobbler"
SERVICE_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_FILE="$SERVICE_DIR/mpd-scrobbler.service"

echo "==> Stopping and disabling systemd user service..."
if command -v systemctl >/dev/null 2>&1; then
    systemctl --user stop mpd-scrobbler.service 2>/dev/null || true
    systemctl --user disable mpd-scrobbler.service 2>/dev/null || true
fi

if [ -f "$SERVICE_FILE" ]; then
    echo "==> Removing service file $SERVICE_FILE..."
    rm -f "$SERVICE_FILE"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl --user daemon-reload 2>/dev/null || true
        systemctl --user reset-failed 2>/dev/null || true
    fi
fi

if [ -f "$BIN_DIR/mpd-scrobbler" ]; then
    echo "==> Removing binary $BIN_DIR/mpd-scrobbler..."
    rm -f "$BIN_DIR/mpd-scrobbler"
fi

echo ""
echo "=================================================================="
echo " mpd-scrobbler service and binary have been uninstalled."
echo "=================================================================="
echo " Note: Your configuration file at $CONFIG_DIR was preserved."
echo " If you wish to completely remove it, run:"
echo "   rm -rf \"$CONFIG_DIR\""
echo "=================================================================="
