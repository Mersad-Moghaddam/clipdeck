# ClipDeck IPC Protocol v1

ClipDeck daemon exposes a Unix Domain Socket at:
`$XDG_RUNTIME_DIR/clipdeck/clipdeck.sock` (mode 0600)
For development profile (`--profile dev`):
`$XDG_RUNTIME_DIR/clipdeck-dev/clipdeck.sock`

## Protocol Framing
- Line-delimited JSON (JSON-Lines). Each request and response is terminated with `\n`.
- Every client request MUST include:
  - `id`: unique request identifier string (e.g. `"req-1"`)
  - `action`: the operation name string
  - `params`: object with method-specific parameters

- Every daemon response includes:
  - `id`: matching the request's `id`
  - `success`: boolean indicating whether the operation succeeded
  - `data`: payload object on success
  - `error`: error message string if `success == false`

- Push events (broadcast from daemon when subscribed):
  - `event`: event name string (e.g. `"item_added"`, `"item_deleted"`, `"item_pinned"`, `"cleared"`, `"paused"`)
  - `data`: payload object

## Operations

### 1. `list`
Fetch clipboard history with pagination.
- Parameters:
  - `limit`: integer (default 50, max 200)
  - `offset`: integer (default 0)
- Response:
  - `items`: array of history items
  - `total`: total count
  - `has_more`: boolean

### 2. `search`
Search items across full text and metadata using SQLite FTS5 trigram.
- Parameters:
  - `query`: string query (supports filter prefixes like `type:image`, `app:firefox`, `pinned:true`)
  - `limit`: integer (default 50)
  - `offset`: integer (default 0)
- Response:
  - `items`: array of matched items
  - `total`: total count

### 3. `pin` / `unpin`
Toggle pinned status of an item.
- Parameters:
  - `id`: string item ID
- Response:
  - `pinned`: boolean

### 4. `delete`
Delete a specific item.
- Parameters:
  - `id`: string item ID
- Response:
  - `deleted`: boolean

### 5. `clear`
Clear unpinned history (pinned items remain unless `include_pinned: true`).
- Parameters:
  - `include_pinned`: boolean (default false)
- Response:
  - `cleared_count`: integer

### 6. `pause` / `resume` / `toggle_pause`
Toggle incognito mode.
- Parameters: none
- Response:
  - `paused`: boolean

### 7. `status`
Query daemon status, memory, item count, and incognito status.
- Response:
  - `version`: string
  - `paused`: boolean
  - `item_count`: integer
  - `pinned_count`: integer
  - `uptime_seconds`: integer

### 8. `transform`
Apply a text transformation on an item or raw string.
- Parameters:
  - `text`: string input
  - `transform`: string (`uppercase`, `lowercase`, `camel`, `snake`, `trim`, `json_pretty`, `base64_encode`, `base64_decode`, `url_encode`, `url_decode`, `sort_lines`, `dedupe_lines`, `persian_digits_to_latin`, `latin_digits_to_persian`, `arabic_to_persian_letters`, `normalize_zwnj`)
- Response:
  - `result`: string

### 9. `subscribe`
Subscribe to live push events.
- Response:
  - `subscribed`: true
