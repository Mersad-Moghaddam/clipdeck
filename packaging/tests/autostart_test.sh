#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/src" "$tmp/pkg"
ln -s "$root" "$tmp/src/clipdeck"
# Exercise the real Arch package() function, not a source-tree fixture.
srcdir="$tmp/src" pkgdir="$tmp/pkg" bash -c '
  source "$1/packaging/aur/clipdeck/PKGBUILD"
  package
' bash "$root"
target="$tmp/pkg/usr/lib/systemd/user/graphical-session.target.wants/clipdeckd.service"
test -L "$target"
test "$(readlink "$target")" = ../clipdeckd.service
test -f "$target"
echo 'PASS: Arch package startup dependency resolves to installed unit'
