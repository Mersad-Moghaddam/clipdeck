# ClipDeck

[![Release](https://img.shields.io/github/v/release/Mersad-Moghaddam/clipdeck?color=blue)](https://github.com/Mersad-Moghaddam/clipdeck/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Mersad-Moghaddam/clipdeck)](go.mod)
[![License](https://img.shields.io/github/license/Mersad-Moghaddam/clipdeck)](LICENSE)
[![CI](https://github.com/Mersad-Moghaddam/clipdeck/actions/workflows/ci.yml/badge.svg)](https://github.com/Mersad-Moghaddam/clipdeck/actions/workflows/ci.yml)

ClipDeck is a keyboard-first clipboard manager daemon and CLI for Omarchy, Hyprland, and modern Wayland desktops.

Built in pure Go with zero CGO dependencies (`modernc.org/sqlite` with FTS5 trigram search), ClipDeck combines persistent clipboard history with automatic content classification, sensitive token protection, and low-latency JSON-lines IPC.

## Features

- **Full-text search**: SQLite FTS5 trigram engine indexes substrings, symbols, and multilingual text. See [measured results](docs/PERFORMANCE.md).
- **Automatic type classification**: Categorizes entries into links, code snippets, colors (HEX, RGB, HSL), JSON, emails, phone numbers, file paths, numbers, and plain text.
- **Privacy and credential protection**:
  - Respects `x-kde-passwordManagerHint` and secret MIME flags from password managers.
  - Detects private keys, JWTs, API tokens, high-entropy secrets, and credit cards with Luhn validation.
  - Masks sensitive entries on ingest and purges them after a configurable TTL (default: 60s).
- **Persian and RTL typography**: Normalizes Persian, Arabic, and Latin digits, handles Arabic Yeh and Kaf substitution, and preserves zero-width non-joiners (ZWNJ).
- **Multiple interfaces**:
  - Interactive terminal fuzzy picker (`clipdeck pick`) compatible with `fzf`, `rofi`, `wofi`, and `bemenu`.
  - Rich desktop HUD overlay for Omarchy via the [`omarchy-clipdeck`](https://github.com/Mersad-Moghaddam/omarchy-clipdeck) Quickshell plugin.
  - Scriptable CLI (`clipdeck list --json`, `clipdeck copy`, `clipdeck status`).
- **Hardened IPC**: Unix domain socket restricted with `0600` file permissions and kernel `SO_PEERCRED` validation to enforce identical caller UID.

## Architecture

```text
wl-paste --watch ───────────┐
hyprctl (active window) ────┼──▶ clipdeckd (Go) ──▶ SQLite (FTS5) + blobs/
                            │        │
                            │   Unix socket (JSON lines, SO_PEERCRED)
                            │        │
   Quickshell overlay: ◀────┴──▶ CLI: clipdeck (list / pick / copy / doctor)
```

## Installation

### Universal installer (recommended)

Install or update to the newest version on any Linux distribution (`amd64` and `arm64`):

```bash
curl -fsSL https://raw.githubusercontent.com/Mersad-Moghaddam/clipdeck/main/packaging/install.sh | bash
```

The script verifies architecture, checks SHA256 checksums, installs binaries atomically into `~/.local/bin/`, sets up the systemd user service, and executes `clipdeck doctor`. Re-running this command at any time updates ClipDeck to the latest release. Pass `--from-source` to compile with Go instead of fetching binaries.

### Standalone desktop app (v0.5.0 development)

The next release adds a standalone Omarchy-style popup and an X11 backend for
i3 and other desktops. **v0.5.0 is not published yet.** The existing Omarchy
overlay is unchanged. See the [desktop support and i3 setup guide](docs/desktop-support.md)
for tested behavior and remaining release gates.

To try the development checkout (Python 3.10+, Go, and the desktop clipboard
dependencies are required):

```bash
# Review the installer and preview its actions before installing.
bash packaging/install.sh --from-source --version 0.5.0 --with-ui --dry-run
bash packaging/install.sh --from-source --version 0.5.0 --with-ui
clipdeck-ui
```

`--with-ui` downloads Qt into a private Python environment, without modifying
system Python or running sudo. `--daemon-only` keeps the existing daemon/plugin
installation path. Ubuntu/Zorin users may need `sudo apt install python3-venv`.
This is a development-install path; use the published packages below for the
current stable release. Installation will not rewrite your i3 configuration.

### Distribution packages

Download and install prebuilt packages for v0.4.3 from the [GitHub releases page](https://github.com/Mersad-Moghaddam/clipdeck/releases). Downloads below fail closed: a nonempty matching checksum entry and successful verification are required before installation.

#### Debian and Ubuntu (`.deb`)

```bash
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/clipdeck_0.4.3_amd64.deb &&
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/SHA256SUMS &&
grep -E '  (\./)?clipdeck_0\.4\.3_amd64\.deb$' SHA256SUMS > package.sha256 &&
test -s package.sha256 && sha256sum -c package.sha256 &&
sudo apt install ./clipdeck_0.4.3_amd64.deb
```
*(For ARM64 systems, substitute `arm64` for `amd64`)*

#### Fedora and RHEL (`.rpm`)

```bash
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/clipdeck-0.4.3-1.x86_64.rpm &&
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/SHA256SUMS &&
grep -E '  (\./)?clipdeck-0\.4\.3-1\.x86_64\.rpm$' SHA256SUMS > package.sha256 &&
test -s package.sha256 && sha256sum -c package.sha256 &&
sudo dnf install ./clipdeck-0.4.3-1.x86_64.rpm
```
*(For ARM64 systems, substitute `aarch64` for `x86_64`)*

#### openSUSE (`.rpm`)

```bash
# Download and verify the RPM using the Fedora block above; use this instead of dnf:
sudo zypper install ./clipdeck-0.4.3-1.x86_64.rpm
```

#### Alpine Linux (`.apk`)

```bash
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/clipdeck_0.4.3_x86_64.apk &&
curl -fLO https://github.com/Mersad-Moghaddam/clipdeck/releases/download/v0.4.3/SHA256SUMS &&
grep -E '  (\./)?clipdeck_0\.4\.3_x86_64\.apk$' SHA256SUMS > package.sha256 &&
test -s package.sha256 && sha256sum -c package.sha256 &&
sudo apk add --allow-untrusted ./clipdeck_0.4.3_x86_64.apk
```

#### Arch Linux

```bash
git clone https://github.com/Mersad-Moghaddam/clipdeck.git /tmp/clipdeck
cd /tmp/clipdeck/packaging/aur/clipdeck && makepkg -si
```

## Quick start

Native systemd packages start the daemon automatically at your next graphical login. In the Omarchy plugin, click **Start & enable at login** to start immediately and enable future logins without typing a terminal command. The companion must already be installed. The portable installer also enables startup by default unless `--no-service` is selected. Non-systemd desktops need a compositor/session autostart entry.

Optional terminal equivalent:

```bash
systemctl --user enable --now clipdeckd.service
```

Verify your environment and system dependencies:

```bash
clipdeck doctor
```

Launch the interactive fuzzy picker in your terminal:

```bash
clipdeck pick
```

## CLI commands

The `clipdeck` executable provides management, diagnostic, and automation commands.

### History and selection

```bash
# Open interactive picker (auto-detects fzf, rofi, wofi, or bemenu)
clipdeck pick

# Open picker with direct paste into active window
clipdeck pick --paste

# Specify selector explicitly
clipdeck pick --tool rofi

# List recent history entries
clipdeck list --limit 20

# Filter history by content type and search term
clipdeck list --type code --query "func main"

# Output structured JSON for scripts
clipdeck list --json --limit 50

# Copy a specific entry by ID to system clipboard
clipdeck copy f93b700bdfa325d1

# Copy plain text without formatting
clipdeck copy f93b700bdfa325d1 --plain
```

### Diagnostics and control

```bash
# Check daemon health, system dependencies, and database integrity
clipdeck doctor

# Display daemon uptime, item counts, and database size
clipdeck status

# Toggle clipboard capture pause (incognito mode)
clipdeck pause toggle

# Explicitly pause or resume capture
clipdeck pause on
clipdeck pause off

# Clear unpinned history
clipdeck clear

# Clear entire history including pinned items
clipdeck clear --include-pinned
```

### Backups and migration

```bash
# Export history to JSON
clipdeck export --output clipboard_backup.json

# Import history from JSON file
clipdeck import clipboard_backup.json

# Import existing clipboard items from cliphist
clipdeck import-cliphist

# Populate database with synthetic test entries (development profile)
clipdeck --profile dev dev seed
```

## Omarchy visual overlay

For Omarchy and Quickshell users, the [`omarchy-clipdeck`](https://github.com/Mersad-Moghaddam/omarchy-clipdeck) plugin provides a native Wayland HUD overlay:

- Default keyboard shortcut: `Super+V`
- Instant search with server-side category tabs (All, Code, Links, Text, Media)
- Direct row quick actions: Copy, Pin, Delete, and multi-item Merge
- Full inspector panel with Markdown rendering and RTL typography

To install the overlay plugin, place the repository in `~/.config/omarchy/plugins/io.github.mersadmoghaddam.clipdeck/`.

## Building from source

### Prerequisites

- Go 1.22 or higher
- `wl-clipboard` (`wl-paste`, `wl-copy`)
- `wtype` (optional, required for simulated direct paste)
- `hyprland` (optional, enables active window tracking and focus paste fallback)

### Build and install

```bash
# Compile binaries into bin/clipdeckd and bin/clipdeck
make build

# Install binaries into ~/.local/bin and install user service
make install
```

### Test suite

```bash
make test    # Run unit tests
make race    # Run race condition detector
make bench   # Run FTS5 indexing and query benchmarks
make integ   # Run end-to-end integration tests
```

## Documentation

- [IPC protocol specification](docs/PROTOCOL.md): JSON-lines framing, methods, params, and event streaming.
- [Desktop compatibility matrix](docs/COMPATIBILITY.md): Compositor and distribution support status.
- [Performance benchmarks](docs/PERFORMANCE.md): Query response timings and memory usage metrics.
- [Release verification checklist](docs/RELEASE_CHECKLIST.md): Quality gates and verification criteria.

## License

MIT License. See [LICENSE](LICENSE) for details.
