#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/home" "$tmp/mock" "$tmp/payload" "$tmp/ClipDeck Apps/bin"
printf '#!/bin/sh\nexit 0\n' >"$tmp/payload/clipdeck"
cp "$tmp/payload/clipdeck" "$tmp/payload/clipdeckd"
tar -czf "$tmp/payload.tar.gz" -C "$tmp/payload" clipdeck clipdeckd
cat >"$tmp/mock/curl" <<'SH'
#!/bin/bash
while [ $# -gt 0 ]; do
 if [ "$1" = -o ]; then target="$2"; shift 2; else url="$1"; shift; fi
done
if [[ "$url" = *.sha256 ]]; then
 printf '%s  clipdeck_0.5.0_linux_amd64.tar.gz\n' "$(sha256sum "$PAYLOAD" | cut -d' ' -f1)" >"$target"
else cp "$PAYLOAD" "$target"; fi
SH
cat >"$tmp/mock/systemctl" <<'SH'
#!/bin/bash
printf '%s\n' "$*" >>"$LOG"
case "$*" in *is-active*|*is-enabled*) exit 1;; *restart*) [[ ${FAIL_START:-0} = 0 ]];; esac
SH
chmod +x "$tmp/mock/"*
export HOME="$tmp/home" PATH="$tmp/mock:$PATH" PAYLOAD="$tmp/payload.tar.gz" LOG="$tmp/log"
prefix="$tmp/ClipDeck Apps"
printf old-daemon >"$prefix/bin/clipdeckd"
printf old-cli >"$prefix/bin/clipdeck"
if FAIL_START=1 bash "$root/packaging/install.sh" --version 0.5.0 --prefix "$prefix" >"$tmp/output" 2>&1; then
 echo 'FAIL: service failure accepted'; exit 1
fi
test "$(cat "$prefix/bin/clipdeckd")" = old-daemon
test "$(cat "$prefix/bin/clipdeck")" = old-cli
test ! -e "$HOME/.config/systemd/user/clipdeckd.service"
cat >"$tmp/mock/mv" <<'SH'
#!/bin/bash
if [[ "$*" = *clipdeck.new* ]]; then kill -TERM "$PPID"; exit 1; fi
exec /usr/bin/mv "$@"
SH
chmod +x "$tmp/mock/mv"
if bash "$root/packaging/install.sh" --version 0.5.0 --no-service --prefix "$prefix" >"$tmp/output" 2>&1; then
 echo 'FAIL: interruption accepted'; exit 1
fi
test "$(cat "$prefix/bin/clipdeckd")" = old-daemon
test "$(cat "$prefix/bin/clipdeck")" = old-cli
rm "$tmp/mock/mv"
bash "$root/packaging/install.sh" --version 0.5.0 --prefix "$prefix" >"$tmp/output" 2>&1
grep -Fq "ExecStart=\"$prefix/bin/clipdeckd\"" "$HOME/.config/systemd/user/clipdeckd.service"
grep -q -- '--user restart clipdeckd.service' "$tmp/log"
echo 'PASS: late service failure rolls back binaries/unit; spaced service executable is quoted'
