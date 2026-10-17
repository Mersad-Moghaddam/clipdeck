#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/home" "$TMP/mock" "$TMP/payload"
printf '#!/bin/sh\nexit 0\n' > "$TMP/payload/clipdeck"
cp "$TMP/payload/clipdeck" "$TMP/payload/clipdeckd"
ln -s /etc/passwd "$TMP/payload/unexpected"
tar -czf "$TMP/payload.tar.gz" -C "$TMP/payload" .
cat > "$TMP/mock/curl" <<'EOF'
#!/usr/bin/env bash
set -e
while [ $# -gt 0 ]; do
 if [ "$1" = -o ]; then target="$2"; shift 2; else url="$1"; shift; fi
done
if [[ "$url" = *.sha256 ]]; then
 hash="$(sha256sum "$PAYLOAD" | cut -d' ' -f1)"
 printf '%s  clipdeck_0.4.0-dev_linux_amd64.tar.gz\n' "$hash" > "$target"
else
 cp "$PAYLOAD" "$target"
fi
EOF
chmod +x "$TMP/mock/curl"
if HOME="$TMP/home" PATH="$TMP/mock:$PATH" PAYLOAD="$TMP/payload.tar.gz" bash "$ROOT/packaging/install.sh" --version 0.4.0-dev --no-service > "$TMP/output" 2>&1; then
 echo 'FAIL: accepted unexpected symlink/archive entry'; exit 1
fi
test ! -e "$TMP/home/.local/bin/clipdeck"
grep -q 'unsafe archive' "$TMP/output"
echo 'Archive safety regression passed'
