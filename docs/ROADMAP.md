# ClipDeck Roadmap (Post 0.1.0 / P2 Backlog)

The following items are defined in PRD §5 as P2 features scheduled for future releases:

## FR-21: Collections and Tags
- Custom user-created collections (e.g. "Work", "Snippets", "Research").
- Free-form tags per item with tag-based filtering (`tag:project-x`).

## FR-22: Encrypted Vault for Sensitive Pinned Items
- Strong encryption at rest (AES-256-GCM / Argon2id key derivation) for sensitive pins.
- Passphrase unlock prompt using polkit or system keychain.

## FR-23: Local Area Network (LAN) Sync
- End-to-end encrypted peer-to-peer sync between devices on the same local network.
- Explicit mutual authentication with zero cloud dependency.

## FR-24: User Transform Plugins
- Extensibility directory at `~/.config/clipdeck/transforms/`.
- User-supplied executable scripts (bash, python, node) receiving content via stdin and returning transformed text on stdout.

## FR-25: Link Page Titles
- Optional, opt-in background fetching of HTML `<title>` tags for link items.
- Strictly disabled by default to uphold ClipDeck's zero-network-traffic privacy guarantee.
