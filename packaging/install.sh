#!/usr/bin/env bash
# ClipDeck installer — Linux Wayland and X11 desktops.
#
#   curl -fsSL https://raw.githubusercontent.com/Mersad-Moghaddam/clipdeck/main/packaging/install.sh | bash
#
# Modes:
#   default        download the prebuilt release tarball for this architecture
#   --from-source  build with Go (requires go >= 1.21) from this checkout or
#                  from a fresh clone of the repo
#
# Options:
#   --prefix DIR     install root (default: ~/.local, binaries -> DIR/bin)
#   --version X.Y.Z  release to install (default: latest)
#   --no-service     don't enable/start the systemd user service
#   --dry-run        print what would happen, touch nothing
#   --uninstall      remove binaries and the user service
#   --with-ui        also install standalone Qt popup in an isolated Python venv
#   --daemon-only    install daemon/CLI only (default; keeps Omarchy integration)
set -euo pipefail

REPO="Mersad-Moghaddam/clipdeck"
PREFIX="${HOME}/.local"
VERSION="latest"
FROM_SOURCE=0
SERVICE=1
DRY_RUN=0
UNINSTALL=0
WITH_UI=0

say()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m warn\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror\033[0m %s\n' "$*" >&2; exit 1; }

run() {
  if [ "$DRY_RUN" = 1 ]; then printf '    [dry-run]'; printf ' %q' "$@"; printf '\n'; return 0; fi
  "$@"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --prefix)      [ $# -ge 2 ] || die "--prefix requires a value"; PREFIX="$2"; shift 2 ;;
    --prefix=*)    PREFIX="${1#*=}"; shift ;;
    --version)     [ $# -ge 2 ] || die "--version requires a value"; VERSION="$2"; shift 2 ;;
    --version=*)   VERSION="${1#*=}"; shift ;;
    --from-source) FROM_SOURCE=1; shift ;;
    --no-service)  SERVICE=0; shift ;;
    --dry-run)     DRY_RUN=1; shift ;;
    --uninstall)   UNINSTALL=1; shift ;;
    --with-ui)     WITH_UI=1; shift ;;
    --daemon-only) WITH_UI=0; shift ;;
    -h|--help)     sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)             die "unknown option: $1 (see --help)" ;;
  esac
done

BINDIR="${PREFIX}/bin"
UNIT_DIR="${HOME}/.config/systemd/user"
UNIT_FILE="${UNIT_DIR}/clipdeckd.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "unsupported architecture: $(uname -m) — build from source with --from-source" ;;
esac

systemctl_user() { systemctl --user "$@"; }

# ---------------------------------------------------------------- uninstall
if [ "$UNINSTALL" = 1 ]; then
  say "Removing ClipDeck"
  HAD_UNIT=0
  [ ! -f "$UNIT_FILE" ] || HAD_UNIT=1
  if [ -f "$UNIT_FILE" ] && command -v systemctl >/dev/null 2>&1; then
    run systemctl_user disable --now clipdeckd.service || true
  fi
  run rm -f "$UNIT_FILE" "$BINDIR/clipdeck" "$BINDIR/clipdeckd"
  run rm -f "$BINDIR/clipdeck-ui" "${PREFIX}/share/applications/clipdeck.desktop"
  run rm -f "$BINDIR/clipdeck-session" "${HOME}/.config/autostart/clipdeck-session.desktop"
  if [ "$HAD_UNIT" = 1 ] && command -v systemctl >/dev/null 2>&1; then
    run systemctl_user daemon-reload || true
  fi
  say "Done. Your history is kept in ~/.local/share/clipdeck and ~/.local/state/clipdeck."
  exit 0
fi

# ---------------------------------------------------------------- resolve version
if [ "$DRY_RUN" = 1 ]; then
  say "Dry run: install ${VERSION} (${ARCH}) into ${PREFIX}; service=${SERVICE}; source=${FROM_SOURCE}; ui=${WITH_UI}. No downloads or files changed."
  exit 0
fi
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fSsL --connect-timeout 5 --max-time 15 "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)" || true
  # Never fall back to a stale hardcoded version — silently installing old
  # binaries is exactly the failure this script exists to prevent.
  [ -n "$VERSION" ] || die "Could not reach the GitHub API to resolve the latest release; pass an explicit --version instead."
fi
say "Installing ClipDeck ${VERSION} (${ARCH}) into ${PREFIX}"

TMP="$(mktemp -d)"
UI_ENV=""
UI_COMMITTED=0
TRANSACTION=0
COMMITTED=0
WAS_ACTIVE=0
WAS_ENABLED=0
declare -a SAVED_PATHS=()
snapshot() {
  local path=$1 index=${#SAVED_PATHS[@]}
  SAVED_PATHS+=("$path")
  if [ -e "$path" ] || [ -L "$path" ]; then cp -a -- "$path" "$TMP/saved-$index"; fi
}
cleanup() {
  local status=$? index path
  if [ "$TRANSACTION" = 1 ] && [ "$COMMITTED" = 0 ]; then
    warn "Installation failed; restoring the previous installation"
    for index in "${!SAVED_PATHS[@]}"; do
      path=${SAVED_PATHS[$index]}
      rm -f -- "$path"
      if [ -e "$TMP/saved-$index" ] || [ -L "$TMP/saved-$index" ]; then cp -a -- "$TMP/saved-$index" "$path"; fi
    done
    rm -f -- "$BINDIR/clipdeck.new" "$BINDIR/clipdeckd.new"
    if [ "$SERVICE" = 1 ] && command -v systemctl >/dev/null 2>&1; then
      systemctl_user daemon-reload || true
      if [ "$WAS_ENABLED" = 0 ]; then systemctl_user disable clipdeckd.service >/dev/null 2>&1 || true; fi
      if [ "$WAS_ACTIVE" = 1 ]; then systemctl_user restart clipdeckd.service || true
      else systemctl_user stop clipdeckd.service >/dev/null 2>&1 || true; fi
    fi
  fi
  rm -rf "$TMP"
  if [ -n "$UI_ENV" ] && [ "$UI_COMMITTED" = 0 ]; then
    rm -rf -- "$UI_ENV"
  fi
  return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [ "$WITH_UI" = 1 ]; then
  command -v python3 >/dev/null 2>&1 || die "The graphical app needs Python 3.10+. Ubuntu/Zorin: sudo apt install python3 python3-venv"
  python3 -c 'import sys; assert sys.version_info >= (3, 10)' || die "Python 3.10 or newer is required for the graphical app"
  say "Standalone UI selected: downloads Qt/PySide6 into a private environment; system Python is not changed."
fi

# ---------------------------------------------------------------- fetch binaries
if [ "$FROM_SOURCE" = 1 ]; then
  command -v go >/dev/null 2>&1 || die "Go is required for --from-source (https://go.dev/dl/)"
  SRC="$REPO_ROOT"
  if [ ! -f "$SRC/go.mod" ]; then
    say "Cloning ${REPO}"
    run git clone --depth 1 "https://github.com/${REPO}.git" "$TMP/src"
    SRC="$TMP/src"
  fi
  say "Building from source"
  (cd "$SRC" && run env CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o "$TMP/clipdeck" ./cmd/clipdeck)
  (cd "$SRC" && run env CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o "$TMP/clipdeckd" ./cmd/clipdeckd)
else
  TARBALL="clipdeck_${VERSION}_linux_${ARCH}.tar.gz"
  # CLIPDECK_RELEASE_BASE_URL lets mirrored/air-gapped setups host the assets.
  RELEASE_BASE="${CLIPDECK_RELEASE_BASE_URL:-https://github.com/${REPO}/releases/download}"
  URL="${RELEASE_BASE}/v${VERSION}/${TARBALL}"
  say "Downloading ${URL}"
  if ! run curl -fSsL --connect-timeout 10 --max-time 300 -o "$TMP/$TARBALL" "$URL"; then
    die "could not download the release tarball.

  - check your network connection (the release lives on GitHub)
  - or host the assets anywhere else and point the installer at them:
      CLIPDECK_RELEASE_BASE_URL=<base> ${0##*/} --version ${VERSION}
  - on minimal systems install CA certificates first:
      apt install ca-certificates   |   dnf install ca-certificates   |   apk add ca-certificates"
  fi
  if curl -fSsL --connect-timeout 5 --max-time 20 -o "$TMP/$TARBALL.sha256" "${URL}.sha256" 2>/dev/null; then
    say "Verifying checksum"
    ( cd "$TMP" && sha256sum -c "$TARBALL.sha256" )
  else
    die "checksum unavailable; refusing unverified release"
  fi
  ARCHIVE_ENTRIES="$(tar -tzf "$TMP/$TARBALL")" || die "unsafe archive: cannot inspect entries"
  [ "$(printf '%s\n' "$ARCHIVE_ENTRIES" | LC_ALL=C sort)" = "$(printf 'clipdeck\nclipdeckd')" ] || die "unsafe archive: unexpected paths"
  ARCHIVE_MODES="$(tar -tvzf "$TMP/$TARBALL" | cut -c1)" || die "unsafe archive: cannot inspect types"
  [ "$ARCHIVE_MODES" = "$(printf '%s\n%s' '-' '-')" ] || die "unsafe archive: only regular binaries allowed"
  run tar --no-same-owner -C "$TMP" -xzf "$TMP/$TARBALL"
fi

# Stage GUI dependencies before changing the installed binaries. --with-ui is
# explicit consent to install the Qt runtime; no sudo/package-manager commands.
if [ "$WITH_UI" = 1 ]; then
  run install -d "${PREFIX}/share/clipdeck"
  UI_ENV=$(mktemp -d "${PREFIX}/share/clipdeck/ui-${VERSION}-XXXXXXXX")
  if ! python3 -m venv "$UI_ENV"; then
    die "Cannot create a Python environment. Ubuntu/Zorin: sudo apt install python3-venv; then retry."
  fi
  if [ "$FROM_SOURCE" = 1 ]; then
    UI_SOURCE="$SRC/desktop"
  else
    UI_WHEEL="clipdeck_ui-${VERSION}-py3-none-any.whl"
    curl -fSsL --connect-timeout 10 --max-time 300 -o "$TMP/$UI_WHEEL" "${RELEASE_BASE}/v${VERSION}/${UI_WHEEL}" || die "Could not download graphical app; existing binaries are unchanged"
    curl -fSsL --connect-timeout 10 --max-time 30 -o "$TMP/SHA256SUMS" "${RELEASE_BASE}/v${VERSION}/SHA256SUMS" || die "GUI checksum manifest unavailable"
    awk -v file="$UI_WHEEL" '$2 == file || $2 == "./" file { print }' "$TMP/SHA256SUMS" > "$TMP/ui.sha256"
    [ "$(wc -l < "$TMP/ui.sha256")" = 1 ] || die "Missing or ambiguous graphical app checksum"
    (cd "$TMP" && sha256sum -c ui.sha256) || die "Graphical app checksum mismatch"
    UI_SOURCE="$TMP/$UI_WHEEL"
  fi
  "$UI_ENV/bin/pip" install "$UI_SOURCE" || die "Qt installation failed; existing binaries are unchanged. Retry with --daemon-only to skip the graphical app."
  QT_QPA_PLATFORM=offscreen QT_QUICK_BACKEND=software "$UI_ENV/bin/clipdeck-ui" --smoke-test || die "Graphical app preflight failed; existing binaries are unchanged"
  "$UI_ENV/bin/python" -m clipdeck_ui.preflight || die "Native Qt preflight failed; existing installation unchanged. Install the desktop libraries listed in docs/desktop-support.md and retry."
  if [ "$FROM_SOURCE" = 1 ]; then
    cp "$SRC/packaging/clipdeck-session" "$TMP/clipdeck-session"
  else
    curl -fSsL --connect-timeout 10 --max-time 30 -o "$TMP/clipdeck-session" "${RELEASE_BASE}/v${VERSION}/clipdeck-session" || die "Session startup helper unavailable"
    awk '$2 == "clipdeck-session" || $2 == "./clipdeck-session" { print }' "$TMP/SHA256SUMS" > "$TMP/session.sha256"
    [ "$(wc -l < "$TMP/session.sha256")" = 1 ] || die "Session helper checksum missing or ambiguous"
    (cd "$TMP" && sha256sum -c session.sha256) || die "Session startup helper checksum mismatch"
  fi
fi

# ---------------------------------------------------------------- install
for path in "$BINDIR/clipdeck" "$BINDIR/clipdeckd" "$BINDIR/clipdeck-ui" "$BINDIR/clipdeck-session" "${PREFIX}/share/applications/clipdeck.desktop" "${HOME}/.config/autostart/clipdeck-session.desktop" "$UNIT_FILE"; do
  snapshot "$path"
done
if [ "$SERVICE" = 1 ] && command -v systemctl >/dev/null 2>&1; then
  if systemctl_user is-active --quiet clipdeckd.service; then WAS_ACTIVE=1; fi
  if systemctl_user is-enabled --quiet clipdeckd.service; then WAS_ENABLED=1; fi
fi
TRANSACTION=1
run install -d "$BINDIR"
run install -m 755 "$TMP/clipdeckd" "$BINDIR/clipdeckd.new"
run install -m 755 "$TMP/clipdeck"  "$BINDIR/clipdeck.new"

run mv -f "$BINDIR/clipdeckd.new" "$BINDIR/clipdeckd" || die "could not install clipdeckd"
run mv -f "$BINDIR/clipdeck.new" "$BINDIR/clipdeck" || die "could not install clipdeck"

if [ "$WITH_UI" = 1 ]; then
  run install -m755 "$TMP/clipdeck-session" "$BINDIR/clipdeck-session"
  ln -sfn "$UI_ENV/bin/clipdeck-ui" "$BINDIR/clipdeck-ui"
  run install -d "${PREFIX}/share/applications"
  # Desktop Entry Exec uses double-quoted argument escaping, not shell syntax.
  UI_LAUNCHER="$BINDIR/clipdeck-ui" python3 - "${PREFIX}/share/applications/clipdeck.desktop" <<'PY'
import os, pathlib, sys
path = os.environ['UI_LAUNCHER']
quoted = '"' + ''.join('\\' + ch if ch in '\\"`$' else ch for ch in path).replace('%', '%%') + '"'
pathlib.Path(sys.argv[1]).write_text('[Desktop Entry]\nType=Application\nName=ClipDeck\nComment=Clipboard history\nExec=' + quoted + '\nIcon=edit-paste\nTerminal=false\nCategories=Utility;\nStartupWMClass=clipdeck-ui\n')
PY
  if [ "$SERVICE" = 1 ]; then
    run install -d "${HOME}/.config/autostart"
    UI_LAUNCHER="$BINDIR/clipdeck-session" python3 - "${HOME}/.config/autostart/clipdeck-session.desktop" <<'PY'
import os, pathlib, sys
path = os.environ['UI_LAUNCHER']
quoted = '"' + ''.join('\\' + ch if ch in '\\"`$' else ch for ch in path).replace('%', '%%') + '"'
pathlib.Path(sys.argv[1]).write_text('[Desktop Entry]\nType=Application\nName=ClipDeck session\nExec=' + quoted + '\nTerminal=false\nNoDisplay=true\n')
PY
  fi
  say "Graphical app installed. Open ClipDeck from your application menu or run: $BINDIR/clipdeck-ui"
fi

if [ "$SERVICE" = 1 ] && command -v systemctl >/dev/null 2>&1; then
  run install -d "$UNIT_DIR"
  if [ -f "$REPO_ROOT/packaging/clipdeckd.service" ]; then
    cp "$REPO_ROOT/packaging/clipdeckd.service" "$TMP/clipdeckd.service"
  else
    cat > "$TMP/clipdeckd.service" <<EOF
[Unit]
Description=ClipDeck Clipboard History Daemon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=/usr/bin/clipdeckd
Restart=always
RestartSec=1s
RuntimeDirectory=clipdeck
StateDirectory=clipdeck

[Install]
WantedBy=graphical-session.target
EOF
  fi
  # Quote systemd's executable token (not shell quoting); escape specifiers.
  executable=${BINDIR//\\/\\\\}/clipdeckd
  executable=${executable//\"/\\\"}
  executable=${executable//%/%%}
  executable=${executable//\$/\$\$}
  while IFS= read -r line; do
    if [[ $line = ExecStart=* ]]; then printf 'ExecStart="%s"\n' "$executable"; else printf '%s\n' "$line"; fi
  done < "$TMP/clipdeckd.service" > "$TMP/quoted.service"
  mv "$TMP/quoted.service" "$TMP/clipdeckd.service"
  run install -m 644 "$TMP/clipdeckd.service" "$UNIT_FILE"
  run systemctl_user daemon-reload
  variables=()
  for name in DISPLAY XAUTHORITY WAYLAND_DISPLAY XDG_SESSION_TYPE XDG_CURRENT_DESKTOP HYPRLAND_INSTANCE_SIGNATURE; do
    if [ -n "${!name:-}" ]; then variables+=("$name"); else run systemctl_user unset-environment "$name"; fi
  done
  if [ ${#variables[@]} -gt 0 ]; then run systemctl_user import-environment "${variables[@]}"; fi
  run systemctl_user enable clipdeckd.service
  run systemctl_user restart clipdeckd.service || die "New daemon failed to start"
else
  warn "systemd not found (or --no-service): start the daemon manually with '${BINDIR}/clipdeckd &'"
fi
COMMITTED=1
UI_COMMITTED=1

# ---------------------------------------------------------------- hints
case ":${PATH}:" in
  *":${BINDIR}:"*) ;;
  *) warn "${BINDIR} is not in your PATH — add it to your shell profile" ;;
esac
if [ "${XDG_SESSION_TYPE:-}" = x11 ] || { [ -z "${WAYLAND_DISPLAY:-}" ] && [ -n "${DISPLAY:-}" ]; }; then
  for dep in xclip xdotool xprop; do
    command -v "$dep" >/dev/null 2>&1 || warn "Missing $dep. Ubuntu/Zorin: sudo apt install xclip xdotool x11-utils; Arch/i3: sudo pacman -S xclip xdotool xorg-xprop"
  done
else
  for dep in wl-paste wtype; do
    command -v "$dep" >/dev/null 2>&1 || warn "'$dep' not found — install wl-clipboard (and wtype for direct paste)"
  done
fi

say "Installed. Run '${BINDIR}/clipdeck doctor' to verify."
if [ "$DRY_RUN" = 0 ] && [ -x "$BINDIR/clipdeck" ]; then
  "$BINDIR/clipdeck" doctor || true
fi
