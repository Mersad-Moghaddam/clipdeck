#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "$0")/.." && pwd)
for tool in Xvfb xclip; do command -v "$tool" >/dev/null || { echo "Missing X11 test dependency: $tool" >&2; exit 1; }; done
tmp=$(mktemp -d)
trap 'if [[ -n ${server:-} ]]; then kill "$server" 2>/dev/null || true; wait "$server" 2>/dev/null || true; fi; rm -rf "$tmp"' EXIT
Xvfb -displayfd 3 -screen 0 1024x768x24 -nolisten tcp 3>"$tmp/display" >"$tmp/log" 2>&1 &
server=$!
for ((i=0;i<100;i++)); do [[ -s "$tmp/display" ]] && break; kill -0 "$server" 2>/dev/null || { cat "$tmp/log" >&2; exit 1; }; sleep .05; done
[[ -s "$tmp/display" ]] || { echo 'Xvfb did not start' >&2; exit 1; }
export DISPLAY=":$(cat "$tmp/display")" XDG_SESSION_TYPE=x11 CLIPDECK_TEST_X11=1
unset WAYLAND_DISPLAY
cd "$root"
go test -count=1 ./internal/desktop -run TestX11RealClipboardRoundTrip
