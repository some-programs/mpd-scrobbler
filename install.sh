#!/usr/bin/env bash
set -euo pipefail

# Directories based on XDG Base Directory specification
BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/mpd-scrobbler"
SERVICE_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_FILE="$SERVICE_DIR/mpd-scrobbler.service"

echo "==> Building mpd-scrobbler..."
go build -o mpd-scrobbler .

echo "==> Installing binary to $BIN_DIR..."
mkdir -p "$BIN_DIR"
install -m 755 mpd-scrobbler "$BIN_DIR/mpd-scrobbler"
rm -f mpd-scrobbler

echo "==> Setting up configuration directory at $CONFIG_DIR..."
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
    echo "    Creating template config at $CONFIG_DIR/config.toml..."
    cp config.example.toml "$CONFIG_DIR/config.toml"
    chmod 600 "$CONFIG_DIR/config.toml"
else
    echo "    Existing configuration found at $CONFIG_DIR/config.toml (preserving)."
fi

echo "==> Installing user systemd service at $SERVICE_FILE..."
mkdir -p "$SERVICE_DIR"

cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=MPD Last.fm Scrobbler Daemon
After=mpd.service network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN_DIR/mpd-scrobbler
Restart=on-failure
RestartSec=5s
Environment="CONFIG=$CONFIG_DIR/config.toml"

[Install]
WantedBy=default.target
EOF

# Ensure user lingering or dbus is available for user systemd
if command -v systemctl >/dev/null 2>&1; then
    echo "==> Reloading systemd user daemon and enabling service..."
    systemctl --user daemon-reload
    systemctl --user enable --now mpd-scrobbler.service || {
        echo "Warning: Could not start service automatically (systemd user bus might not be active in this session)."
    }
fi

echo ""
echo "=================================================================="
echo " mpd-scrobbler has been successfully installed and configured!"
echo "=================================================================="
echo ""
echo " Binary:  $BIN_DIR/mpd-scrobbler"
echo " Config:  $CONFIG_DIR/config.toml"
echo " Service: $SERVICE_FILE"
echo ""
echo " Next steps:"
echo " 1. Edit your configuration file with your Last.fm credentials:"
echo "    nano \"$CONFIG_DIR/config.toml\""
echo ""
echo "    Or generate a session key interactively:"
echo "    $BIN_DIR/mpd-scrobbler -lastfm.api-key=<KEY> -lastfm.api-secret=<SECRET> -lastfm.auth"
echo ""
echo " 2. Restart or check the service:"
echo "    systemctl --user restart mpd-scrobbler.service"
echo "    systemctl --user status mpd-scrobbler.service"
echo "    journalctl --user -u mpd-scrobbler -f"
echo "=================================================================="
