#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/home" "$TMP/mock"
cat > "$TMP/mock/curl" <<'EOF'
#!/bin/sh
echo called >> "$NETWORK_LOG"
exit 1
EOF
chmod +x "$TMP/mock/curl"
export NETWORK_LOG="$TMP/network"
HOME="$TMP/home" PATH="$TMP/mock:$PATH" bash "$ROOT/packaging/install.sh" --version 0.4.0 --dry-run --no-service > "$TMP/output" 2>&1
test ! -e "$TMP/network" || { echo 'FAIL: dry-run made network requests'; exit 1; }
if HOME="$TMP/home" bash "$ROOT/packaging/install.sh" --version > "$TMP/error" 2>&1; then
  echo 'FAIL: accepted missing version'; exit 1
fi
grep -q 'requires a value' "$TMP/error"
mkdir -p "$TMP/payload"
printf '#!/bin/sh\nexit 0\n' > "$TMP/payload/clipdeck"
cp "$TMP/payload/clipdeck" "$TMP/payload/clipdeckd"
tar -czf "$TMP/payload.tar.gz" -C "$TMP/payload" clipdeck clipdeckd
cat > "$TMP/mock/curl" <<'EOF'
#!/bin/bash
while [ $# -gt 0 ]; do
 if [ "$1" = -o ]; then target="$2"; shift 2; else url="$1"; shift; fi
done
if [[ "$url" = *.sha256 ]]; then
 printf '%s  clipdeck_0.4.0-dev_linux_amd64.tar.gz\n' "$(sha256sum "$PAYLOAD" | cut -d' ' -f1)" > "$target"
else cp "$PAYLOAD" "$target"; fi
EOF
cat > "$TMP/mock/mv" <<'EOF'
#!/bin/bash
if [[ "$*" = *clipdeck.new* ]] && [[ "$*" != *clipdeckd.new* ]]; then exit 1; fi
exec /usr/bin/mv "$@"
EOF
chmod +x "$TMP/mock/curl" "$TMP/mock/mv"
mkdir -p "$TMP/home/.local/bin"
printf 'old-cli' > "$TMP/home/.local/bin/clipdeck"
printf 'old-daemon' > "$TMP/home/.local/bin/clipdeckd"
if HOME="$TMP/home" PATH="$TMP/mock:$PATH" PAYLOAD="$TMP/payload.tar.gz" bash "$ROOT/packaging/install.sh" --version 0.4.0-dev --no-service > "$TMP/output" 2>&1; then
 echo 'FAIL: replacement-failure injection did not abort'; exit 1
fi
test "$(cat "$TMP/home/.local/bin/clipdeck")" = old-cli
test "$(cat "$TMP/home/.local/bin/clipdeckd")" = old-daemon

# Verify uninstall removes binaries but preserves history
mkdir -p "$TMP/home/.local/share/clipdeck"
printf 'preserve-history' > "$TMP/home/.local/share/clipdeck/clipdeck.db"
HOME="$TMP/home" bash "$ROOT/packaging/install.sh" --uninstall > "$TMP/uninst_out" 2>&1
test ! -e "$TMP/home/.local/bin/clipdeck"
test ! -e "$TMP/home/.local/bin/clipdeckd"
test "$(cat "$TMP/home/.local/share/clipdeck/clipdeck.db")" = preserve-history

echo 'Installer dry-run, argument, rollback, and uninstall tests passed'
