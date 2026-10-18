#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"
cat >"$tmp/bin/systemctl" <<'SH'
#!/bin/sh
printf '%s\n' "$*" >>"$LOG"
SH
chmod +x "$tmp/bin/systemctl"
LOG="$tmp/log" PATH="$tmp/bin:$PATH" DISPLAY=:88 XAUTHORITY=/example XDG_SESSION_TYPE=x11 API_TOKEN=private bash "$root/packaging/clipdeck-session"
grep -q -- '--user import-environment' "$tmp/log"
grep -q 'DISPLAY' "$tmp/log"
! grep -q 'API_TOKEN\|private' "$tmp/log"
grep -q -- '--user start clipdeckd.service' "$tmp/log"
echo 'PASS: session startup imports graphical environment only and starts user service'
