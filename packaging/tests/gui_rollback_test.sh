#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/home" "$tmp/mock" "$tmp/payload" "$tmp/prefix/bin"
printf '#!/bin/sh\nexit 0\n' >"$tmp/payload/clipdeck"
cp "$tmp/payload/clipdeck" "$tmp/payload/clipdeckd"
tar -czf "$tmp/payload.tar.gz" -C "$tmp/payload" clipdeck clipdeckd
printf old-daemon >"$tmp/prefix/bin/clipdeckd"
printf old-cli >"$tmp/prefix/bin/clipdeck"
cat >"$tmp/mock/curl" <<'SH'
#!/bin/bash
while [ $# -gt 0 ]; do
 if [ "$1" = -o ]; then target="$2"; shift 2; else url="$1"; shift; fi
done
if [[ "$url" = *.sha256 ]]; then
 printf '%s  clipdeck_0.5.0_linux_amd64.tar.gz\n' "$(sha256sum "$PAYLOAD" | cut -d' ' -f1)" >"$target"
else cp "$PAYLOAD" "$target"; fi
SH
cat >"$tmp/mock/python3" <<'SH'
#!/bin/bash
if [[ "$1" = -m && "$2" = venv ]]; then
 mkdir -p "$3/bin"
 printf '#!/bin/sh\nexit 1\n' >"$3/bin/pip"
 chmod +x "$3/bin/pip"
fi
SH
chmod +x "$tmp/mock/"*
if HOME="$tmp/home" PATH="$tmp/mock:$PATH" PAYLOAD="$tmp/payload.tar.gz" bash "$root/packaging/install.sh" --version 0.5.0 --with-ui --no-service --prefix "$tmp/prefix" >"$tmp/log" 2>&1; then
 echo 'FAIL: GUI failure accepted'; exit 1
fi
test "$(cat "$tmp/prefix/bin/clipdeckd")" = old-daemon
test "$(cat "$tmp/prefix/bin/clipdeck")" = old-cli
# Failed staging must not leave a partially installed Qt environment behind.
test -z "$(find "$tmp/prefix/share/clipdeck" -mindepth 1 -maxdepth 1 -type d -print)"
echo 'PASS: failed GUI staging preserves old binaries and removes partial environment'
