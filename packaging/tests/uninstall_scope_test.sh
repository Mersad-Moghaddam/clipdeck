#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/home"
printf '#!/bin/sh\nprintf touched >"$MARKER"\n' >"$tmp/bin/systemctl"
chmod +x "$tmp/bin/systemctl"
HOME="$tmp/home" MARKER="$tmp/touched" PATH="$tmp/bin:$PATH" bash "$root/packaging/install.sh" --prefix "$tmp/prefix" --uninstall >/dev/null
test ! -e "$tmp/touched"
echo 'PASS: unrelated user service untouched when installer unit is absent'
