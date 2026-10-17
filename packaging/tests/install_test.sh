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
HOME="$TMP/home" PATH="$TMP/mock:$PATH" bash "$ROOT/packaging/install.sh" --version 0.3.1 --dry-run --no-service > "$TMP/output" 2>&1
test ! -e "$TMP/network" || { echo 'FAIL: dry-run made network requests'; exit 1; }
if HOME="$TMP/home" bash "$ROOT/packaging/install.sh" --version > "$TMP/error" 2>&1; then
  echo 'FAIL: accepted missing version'; exit 1
fi
grep -q 'requires a value' "$TMP/error"
echo 'Installer dry-run and argument tests passed'
