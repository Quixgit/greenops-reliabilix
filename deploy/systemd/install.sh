#!/usr/bin/env bash
# Installs and starts the stand's user units. Run as the user that owns ~/reliabilix-stand and ~/.config/reliabilix.
set -euo pipefail
src="$(cd "$(dirname "$0")" && pwd)"
dst="$HOME/.config/systemd/user"
for n in api worker scheduler web; do
  [ -f "$HOME/.config/reliabilix/$n.env" ] || { echo "missing ~/.config/reliabilix/$n.env"; exit 1; }
done
mkdir -p "$dst"
install -m 0644 "$src"/reliabilix-*.service "$dst/"
systemctl --user daemon-reload
loginctl enable-linger "$USER" 2>/dev/null || echo "note: could not enable lingering; services stop at logout"
systemctl --user enable --now reliabilix-api reliabilix-worker reliabilix-scheduler reliabilix-web
