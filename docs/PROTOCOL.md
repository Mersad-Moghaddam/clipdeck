# ClipDeck IPC Protocol v1

The ClipDeck daemon serves a Unix Domain Socket at:

```
$XDG_RUNTIME_DIR/clipdeck/clipdeck.sock   (mode 0600)
```

Development profile (`--profile dev`): `$XDG_RUNTIME_DIR/clipdeck-dev/clipdeck.sock`.
If `XDG_RUNTIME_DIR` is unset the daemon falls back to `/tmp/run-clipdeck/`.
Every socket write is bounded by a 2-second deadline; slow or wedged clients
are dropped by the daemon.

## Framing

Line-delimited JSON (JSON-Lines): every request, response and event is one
line terminated by `\n`.

Request:

```json
{"id": "req-1", "method": "status", "params": {}}
```

- `id`: unique string; echoed back on the matching response
- `method`: operation name (see below)
- `params`: object, method-specific (omit or `{}` when empty)

Response:

```json
{"id": "req-1", "result": {...}}
```

```json
{"id": "req-1", "error": {"code": -32004, "message": "Item not found"}}
```

- `result` present when the call succeeds, `error` when it fails
- error codes: `-32700` parse error, `-32601` unknown method,
  `-32602` invalid params, `-32000` store/IO failure,
  `-32004` item not found

Events (one JSON object per line, has `"event"` instead of `"id"`, clients
skip lines that don't match their pending request id):

```json
{"event": "item_added", "data": { ... full item ... }}
```

| Event            | Payload                                     |
|------------------|---------------------------------------------|
| `item_added`     | full item (also fired on dedupe re-copy bump) |
| `item_deleted`   | `{id}`                                      |
| `pin_changed`    | `{id, pinned}`                              |
| `history_cleared`| `{deleted}`                                 |
| `items_purged`   | `{count}` (expired sensitive items)         |
| `pause_changed`  | `{paused}`                                  |

## Item shape

```json
{
  "id": "c9282aef…",
  "hash": "…",
  "type": "text|code|json|url|email|phone|color|image|file|card",
  "mime": "text/plain",
  "content": "full text (images: blob accessor path)",
  "preview": "short preview line (masked when sensitive)",
  "rich_preview": "HTML-like markup with class=\"t-…\" spans (absent for sensitive items)",
  "blob_path": "/…/blobs/<sha256>.png",
  "thumbnail_path": "/…/thumbs/<sha256>_thumb.png",
  "source_app": "kitty",
  "pinned": false,
  "pinned_at": 0,
  "sensitive": false,
  "masked_content": "",
  "expires_at": 0,
  "size_bytes": 123,
  "created_at": "2026-10-17T09:41:00+03:30",
  "updated_at": "2026-10-17T09:41:00+03:30"
}
```

In `list`/`search` responses, `content` is capped at 16 KiB and
`rich_preview` at 8 KiB so a streaming page can never exceed the daemon's
10 MiB per-line scan limit; the `get` RPC always returns the full content.

## Methods

### `ping`
→ `{pong: true}`

### `status`
Params: none.
→ `{version, paused, total_items, pinned_items, db_size_bytes, uptime_seconds}`

### `list`
Params: `{limit (default 50), offset (default 0)}`.
→ `{items[], total}` ordered `pinned DESC, updated_at DESC`.

### `search`
Params: `{query, limit (default 50), offset}`.
Query prefixes: `type:<type>`, `app:<substring>`, plus the bare word
`pinned` (or `pinned:true`) to restrict to pinned items. Free text runs
through the FTS5 trigram index; queries shorter than 3 characters fall back
to a substring `LIKE` match so one- and two-character prefixes work.
→ `{items[], total}`

### `get`
Params: `{id}`.
→ `{item}` (full content, no caps). Error `-32004` for unknown ids.

### `pin`
Params: `{id, pinned}` (explicit boolean, replaces `pin`/`unpin` methods).
→ `{success: true}`; emits `pin_changed`.

### `delete`
Params: `{id}`.
→ `{success: true}`; emits `item_deleted`. Blob/thumbnail files are reaped
only once the last referencing item is gone.

### `clear`
Params: `{include_pinned (default false)}`.
→ `{success: true, deleted}`; emits `history_cleared`.

### `pause`
Params: `{paused}`.
→ `{paused}`; emits `pause_changed`. State is persisted under
`$XDG_STATE_HOME/clipdeck/paused` and restored on daemon restart.

### `copy`
Params: `{id, plain (bool), transform (optional action name)}`.
Copies the item's content (or image blob) to the clipboard.

### `paste`
Params: `{id, plain, transform}`.
Copy plus a direct paste dispatch into the focused window
(`{success: true}`).

### `transform`
Params: `{id (item), text (raw — use one of the two), action}`.
→ `{text}` — transformed result. Actions: `uppercase`, `lowercase`,
`title`, `camel`, `snake`, `kebab`, `trim`, `json_pretty`, `base64_encode`,
`base64_decode`, `url_encode`, `url_decode`, `sort_lines`, `dedupe_lines`,
`persian_digits_to_latin`, `latin_digits_to_persian`,
`arabic_digits_to_persian`, `arabic_to_persian_letters`, `normalize_zwnj`,
`persian_all`.

### `merge`
Params: `{ids[], separator (default "\n"), action: "preview"|"copy"|"paste"}`.
→ `{text}` of the merged content; copies (and optionally dispatches paste)
as requested.

## Clients

- **CLI** (`clipdeck`): `status`, `doctor`, `clear`, `export`, `import`,
  `import-cliphist`, `pause`, `dev seed` (dev profile only), `version`.
- **Omarchy plugin** connects to the same socket and consumes every event
  above plus `status`/`pause` replies (see `omarchy-clipdeck`).
# Additive query API (v0.4 development)

`query` accepts `{query,types,pinned_only,limit,offset}`. Limit defaults to 50,
maximum 200; offset must be nonnegative. Explicit type filters intersect
query-derived filters and are applied before pagination. Returns
`{items,has_more}`; it does not pretend to provide an exact count. Legacy
list/search retain their total semantics. `status.capabilities` includes
`query` when available.

Sensitive query rows replace content and preview with masked_content, clear
rich previews and blob/thumbnail paths. Trusted local `get`/transform/export
access remains a separate existing policy; this is display protection, not
isolation against another process running as the same user.

Paste responses additionally report `copied:true,paste_requested:true`.
`paste_failed` events contain the item ID and a generic manual-paste hint.
They do not confirm target-application delivery and are not yet correlated to
a unique request; clients must not interpret `success` as delivered input.
