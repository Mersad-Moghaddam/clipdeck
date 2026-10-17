#!/usr/bin/env bash
# ClipDeck installer — any Linux distro, any Wayland desktop.
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
set -euo pipefail

REPO="Mersad-Moghaddam/clipdeck"
PREFIX="${HOME}/.local"
VERSION="latest"
FROM_SOURCE=0
SERVICE=1
DRY_RUN=0
UNINSTALL=0

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
    -h|--help)     sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
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
  if command -v systemctl >/dev/null 2>&1; then
    run systemctl_user disable --now clipdeckd.service || true
  fi
  run rm -f "$UNIT_FILE" "$BINDIR/clipdeck" "$BINDIR/clipdeckd"
  if command -v systemctl >/dev/null 2>&1; then
    run systemctl_user daemon-reload || true
  fi
  say "Done. Your history is kept in ~/.local/share/clipdeck and ~/.local/state/clipdeck."
  exit 0
fi

# ---------------------------------------------------------------- resolve version
if [ "$DRY_RUN" = 1 ]; then
  say "Dry run: install ${VERSION} (${ARCH}) into ${PREFIX}; service=${SERVICE}; source=${FROM_SOURCE}. No downloads or files changed."
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
trap 'rm -rf "$TMP"' EXIT

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
  run env CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o "$TMP/clipdeck"  "${SRC}/cmd/clipdeck"
  run env CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" -o "$TMP/clipdeckd" "${SRC}/cmd/clipdeckd"
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
  run tar --no-same-owner -C "$TMP" -xzf "$TMP/$TARBALL"
fi

# ---------------------------------------------------------------- install
run install -d "$BINDIR"
run install -m 755 "$TMP/clipdeckd" "$BINDIR/clipdeckd"
run install -m 755 "$TMP/clipdeck"  "$BINDIR/clipdeck"

if [ "$SERVICE" = 1 ] && command -v systemctl >/dev/null 2>&1; then
  run install -d "$UNIT_DIR"
  if [ -f "$REPO_ROOT/packaging/clipdeckd.service" ]; then
    sed "s|/usr/bin/clipdeckd|${BINDIR}/clipdeckd|" \
      "$REPO_ROOT/packaging/clipdeckd.service" > "$TMP/clipdeckd.service"
  else
    cat > "$TMP/clipdeckd.service" <<EOF
[Unit]
Description=ClipDeck Clipboard History Daemon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=${BINDIR}/clipdeckd
Restart=always
RestartSec=1s
RuntimeDirectory=clipdeck
StateDirectory=clipdeck

[Install]
WantedBy=graphical-session.target
EOF
  fi
  run install -m 644 "$TMP/clipdeckd.service" "$UNIT_FILE"
  run systemctl_user daemon-reload || true
  run systemctl_user enable --now clipdeckd.service || \
    warn "could not start the service now — it will start with your next login"
else
  warn "systemd not found (or --no-service): start the daemon manually with '${BINDIR}/clipdeckd &'"
fi

# ---------------------------------------------------------------- hints
case ":${PATH}:" in
  *":${BINDIR}:"*) ;;
  *) warn "${BINDIR} is not in your PATH — add it to your shell profile" ;;
esac
for dep in wl-paste wtype; do
  command -v "$dep" >/dev/null 2>&1 || warn "'$dep' not found — install wl-clipboard (and wtype for direct paste)"
done

say "Installed. Run '${BINDIR}/clipdeck doctor' to verify."
if [ "$DRY_RUN" = 0 ] && [ -x "$BINDIR/clipdeck" ]; then
  "$BINDIR/clipdeck" doctor || true
fi
