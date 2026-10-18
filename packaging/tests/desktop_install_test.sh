#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname "$0")/../.." && pwd)
output=$(bash "$root/packaging/install.sh" --with-ui --dry-run --version 0.5.0)
grep -q 'ui=1' <<<"$output"
output=$(bash "$root/packaging/install.sh" --daemon-only --dry-run --version 0.5.0)
grep -q 'ui=0' <<<"$output"
output=$(XDG_SESSION_TYPE=x11 bash "$root/packaging/install.sh" --help)
grep -q -- '--with-ui' <<<"$output"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/prefix/bin" "$tmp/prefix/share/applications" "$tmp/home/.local/share/clipdeck"
touch "$tmp/prefix/bin/clipdeck-ui" "$tmp/prefix/share/applications/clipdeck.desktop" "$tmp/home/.local/share/clipdeck/keep"
HOME="$tmp/home" bash "$root/packaging/install.sh" --prefix "$tmp/prefix" --uninstall >/dev/null 2>&1
test ! -e "$tmp/prefix/bin/clipdeck-ui"
test ! -e "$tmp/prefix/share/applications/clipdeck.desktop"
test -e "$tmp/home/.local/share/clipdeck/keep"
mkdir -p "$tmp/home/.config/autostart"
touch "$tmp/home/.config/autostart/clipdeck-session.desktop" "$tmp/prefix/bin/clipdeck-session"
HOME="$tmp/home" bash "$root/packaging/install.sh" --prefix "$tmp/prefix" --uninstall >/dev/null 2>&1
test ! -e "$tmp/home/.config/autostart/clipdeck-session.desktop"
test ! -e "$tmp/prefix/bin/clipdeck-session"
echo 'PASS: explicit GUI/daemon installer modes and help'
