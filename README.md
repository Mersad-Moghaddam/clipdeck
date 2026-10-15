# ClipDeck

**ClipDeck** is a fast, keyboard-first clipboard manager daemon and CLI for Omarchy 4 and Wayland/Hyprland desktop environments.

Built in pure Go with zero CGO dependencies (`modernc.org/sqlite` with FTS5 trigram search), ClipDeck provides persistent clipboard history, intelligent content detection, sensitive data masking/purging, and a low-latency JSON-lines IPC protocol for UI integrations (such as the `omarchy-clipdeck` Quickshell plugin).

---

## Features

- **Blazing Fast Search:** Sub-30ms search across 10,000+ clipboard items using SQLite FTS5 with trigram tokenization (supports Persian, Arabic, and Latin script substring search).
- **Intelligent Type Detection:** Automatically categorizes entries into links, code, colors, JSON, email, phone numbers, file paths, numbers, and plain text.
- **Privacy & Sensitive Data Protection:**
  - Detects `x-kde-passwordManagerHint` and secret MIME flags from password managers.
  - Automatically identifies private keys, JWTs, API keys, high-entropy tokens, and credit card numbers (with Luhn validation).
  - Automatically masks sensitive items and purges them after a configurable TTL (default 60s).
- **Persian & RTL Support:** Native digit conversion (Persian/Latin/Arabic), Arabic yeh/kaf normalization, and zero-width non-joiner (ZWNJ) normalization.
- **Daemon & CLI:**
  - Lightweight background daemon (`clipdeckd`) with under 20MB idle RSS and near-zero CPU usage.
  - Full CLI (`clipdeck`) featuring `status`, `doctor`, `dev seed`, `clear`, `export`, and `import`.
- **Hardened IPC:** Unix domain socket restricted with `0600` file permissions and kernel-level `SO_PEERCRED` validation (same UID enforcement).

---

## Architecture

```
wl-paste --watch ───────────┐
Hyprland IPC (active window)├──▶ clipdeckd (Go) ──▶ SQLite (FTS5) + blobs/
                            │        │
                            │   Unix socket (JSON lines)
                            │        │
   QML plugin (Quickshell): ◀────────┴──▶ CLI: clipdeck
```

---

## Install

Prebuilt packages for v0.2.0 are attached to every
[GitHub release](https://github.com/Mersad-Moghaddam/clipdeck/releases):

| System | Command |
|---|---|
| Debian / Ubuntu | `sudo apt install ./clipdeck_0.2.0_amd64.deb` |
| Fedora / RHEL / openSUSE | `sudo dnf install ./clipdeck-0.2.0-1.x86_64.rpm` |
| Alpine | `sudo apk add --allow-untrusted ./clipdeck_0.2.0_x86_64.apk` |
| Arch (AUR) | `yay -S clipdeck` |
| Any distro (portable) | `curl -fsSL https://raw.githubusercontent.com/Mersad-Moghaddam/clipdeck/main/packaging/install.sh \| bash` |

The installer detects your architecture, can build from source
(`--from-source`), installs into `~/.local` (override with `--prefix`), wires
up the systemd user service, and runs `clipdeck doctor`. The daemon is pure Go
(CGO-free), so the same binaries run on any distro of the matching
architecture — glibc and musl included.

---

## Building from Source

### Prerequisites
- Go 1.22+
- `wl-clipboard` (`wl-paste`, `wl-copy`)
- `wtype` (optional, for direct paste)
- `hyprland` (optional, for window context and direct paste fallback)

### Build
```bash
make build
```
Binaries will be placed in `bin/clipdeckd` and `bin/clipdeck`.

### Run Tests & Verification
```bash
make test    # Unit tests
make race    # Race detector
make bench   # FTS5 search benchmark
make integ   # End-to-end integration tests
```

---

## Usage

### Running the Daemon
```bash
# Run in development profile (uses ~/.local/share/clipdeck-dev/)
clipdeckd --profile dev

# Run in production
systemctl --user enable --now clipdeckd.service
```

### CLI Commands
```bash
# Diagnostic check of system dependencies and directories
clipdeck doctor

# View daemon and database status
clipdeck status

# Populate database with synthetic test items (useful for development)
clipdeck --profile dev dev seed

# Clear history (unpinned items)
clipdeck clear

# Clear history including pinned items
clipdeck clear --include-pinned

# Export history to JSON
clipdeck export --output backup.json

# Import history from JSON
clipdeck import backup.json

# Import existing entries from cliphist
clipdeck import-cliphist

# Pause / Resume capture (Incognito mode)
clipdeck pause toggle
```

---

## License

MIT License — see [LICENSE](LICENSE).
