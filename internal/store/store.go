package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type Item struct {
	ID            string    `json:"id"`
	Hash          string    `json:"hash"`
	Type          string    `json:"type"`
	Mime          string    `json:"mime"`
	Content       string    `json:"content"`
	Preview       string    `json:"preview"`
	RichPreview   string    `json:"rich_preview"`
	BlobPath      string    `json:"blob_path,omitempty"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`
	SourceApp     string    `json:"source_app"`
	Pinned        bool      `json:"pinned"`
	PinnedAt      int64     `json:"pinned_at"`
	Sensitive     bool      `json:"sensitive"`
	MaskedContent string    `json:"masked_content,omitempty"`
	ExpiresAt     int64     `json:"expires_at"` // Unix seconds
	SizeBytes     int64     `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Store struct {
	db        *sql.DB
	dataDir   string
	blobsDir  string
	thumbsDir string
	mu        sync.RWMutex
}

func Open(dataDir string) (*Store, error) {
	absDataDir, err := filepath.Abs(dataDir)
	if err == nil {
		dataDir = absDataDir
	} else {
		dataDir = filepath.Clean(dataDir)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	blobsDir := filepath.Join(dataDir, "blobs")
	if err := os.MkdirAll(blobsDir, 0700); err != nil {
		return nil, fmt.Errorf("create blobs dir: %w", err)
	}

	thumbsDir := filepath.Join(dataDir, "thumbs")
	if err := os.MkdirAll(thumbsDir, 0700); err != nil {
		return nil, fmt.Errorf("create thumbs dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "clipdeck.db")
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Ensure permissions 0600 on DB files
	_ = os.Chmod(dbPath, 0600)

	s := &Store{
		db:        db,
		dataDir:   dataDir,
		blobsDir:  blobsDir,
		thumbsDir: thumbsDir,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			hash TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			mime TEXT NOT NULL,
			content TEXT,
			preview TEXT,
			rich_preview TEXT,
			blob_path TEXT,
			thumbnail_path TEXT,
			source_app TEXT,
			pinned INTEGER DEFAULT 0,
			pinned_at INTEGER DEFAULT 0,
			sensitive INTEGER DEFAULT 0,
			masked_content TEXT,
			expires_at INTEGER DEFAULT 0,
			size_bytes INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_items_hash ON items(hash);`,
		`CREATE INDEX IF NOT EXISTS idx_items_pinned_updated ON items(pinned, updated_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_items_recent_stable ON items(pinned DESC, updated_at DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_items_expires_at ON items(expires_at) WHERE expires_at > 0;`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
			content,
			preview,
			source_app,
			type,
			content='items',
			content_rowid='rowid',
			tokenize='trigram'
		);`,
		`CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, content, preview, source_app, type)
			VALUES (new.rowid, new.content, new.preview, new.source_app, new.type);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, content, preview, source_app, type)
			VALUES('delete', old.rowid, old.content, old.preview, old.source_app, old.type);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, content, preview, source_app, type)
			VALUES('delete', old.rowid, old.content, old.preview, old.source_app, old.type);
			INSERT INTO items_fts(rowid, content, preview, source_app, type)
			VALUES (new.rowid, new.content, new.preview, new.source_app, new.type);
		END;`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec migration (%s): %w", q, err)
		}
	}

	return nil
}

func (s *Store) SaveItem(item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	// Check if an item with this hash already exists
	var existingID string
	var isPinned bool
	var pinnedAt int64
	err := s.db.QueryRow("SELECT id, pinned, pinned_at FROM items WHERE hash = ?", item.Hash).Scan(&existingID, &isPinned, &pinnedAt)
	if err == nil {
		// Existing item: update its timestamp and bump to top, preserving pin state
		item.ID = existingID
		item.Pinned = isPinned
		item.PinnedAt = pinnedAt

		query := `UPDATE items SET
			type = ?, mime = ?, content = ?, preview = ?, rich_preview = ?,
			blob_path = ?, thumbnail_path = ?, source_app = ?,
			sensitive = ?, masked_content = ?, expires_at = ?, size_bytes = ?,
			updated_at = ?
			WHERE id = ?`
		_, err = s.db.Exec(query,
			item.Type, item.Mime, item.Content, item.Preview, item.RichPreview,
			item.BlobPath, item.ThumbnailPath, item.SourceApp,
			boolToInt(item.Sensitive), item.MaskedContent, item.ExpiresAt, item.SizeBytes,
			item.UpdatedAt.UnixMilli(), item.ID,
		)
		return err
	}

	if item.ID == "" {
		item.ID = hex.EncodeToString(sha256Hash([]byte(fmt.Sprintf("%s:%d", item.Hash, now.UnixNano()))))[:16]
	}

	query := `INSERT INTO items (
		id, hash, type, mime, content, preview, rich_preview,
		blob_path, thumbnail_path, source_app, pinned, pinned_at,
		sensitive, masked_content, expires_at, size_bytes, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		item.ID, item.Hash, item.Type, item.Mime, item.Content, item.Preview, item.RichPreview,
		item.BlobPath, item.ThumbnailPath, item.SourceApp, boolToInt(item.Pinned), item.PinnedAt,
		boolToInt(item.Sensitive), item.MaskedContent, item.ExpiresAt, item.SizeBytes,
		item.CreatedAt.UnixMilli(), item.UpdatedAt.UnixMilli(),
	)
	return err
}

func (s *Store) SaveBlob(data []byte, ext string) (string, error) {
	if strings.ContainsAny(ext, "/\\\x00") {
		return "", fmt.Errorf("invalid blob extension")
	}
	hashHex := hashContent(string(data))
	fileName := hashHex
	if ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		fileName += ext
	}

	fullPath := filepath.Join(s.blobsDir, fileName)
	if fi, err := os.Lstat(fullPath); err == nil {
		if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("blob target is not a regular file")
		}
		return fullPath, nil // already exists
	} else if !os.IsNotExist(err) {
		return "", err
	}

	tmp, err := os.CreateTemp(s.blobsDir, ".blob-*")
	if err != nil {
		return "", err
	}
	tmpFile := tmp.Name()
	defer os.Remove(tmpFile)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	if err := os.Rename(tmpFile, fullPath); err != nil {
		_ = os.Remove(tmpFile)
		return "", err
	}

	return fullPath, nil
}

func (s *Store) ListItems(limit, offset int) ([]Item, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT
		id, hash, type, mime, content, preview, rich_preview,
		blob_path, thumbnail_path, source_app, pinned, pinned_at,
		sensitive, masked_content, expires_at, size_bytes, created_at, updated_at
		FROM items
		ORDER BY pinned DESC, updated_at DESC
		LIMIT ? OFFSET ?`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *Store) SearchItems(query string, limit, offset int) ([]Item, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.ListItems(limit, offset)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var typeFilter, appFilter string
	var pinnedOnly bool
	terms := []string{}

	parts := strings.Fields(query)
	for _, p := range parts {
		if strings.HasPrefix(strings.ToLower(p), "type:") {
			typeFilter = strings.ToLower(p[len("type:"):])
		} else if strings.HasPrefix(strings.ToLower(p), "app:") {
			appFilter = strings.ToLower(p[len("app:"):])
		} else if strings.EqualFold(p, "pinned") || strings.EqualFold(p, "pinned:true") {
			pinnedOnly = true
		} else {
			terms = append(terms, p)
		}
	}

	freeText := strings.Join(terms, " ")

	// FTS5's trigram tokenizer needs at least 3 runes; a 1-2 rune query can
	// never match and silently returns nothing. Route short queries to the
	// LIKE fallback so one-character prefixes stay useful.
	if freeText != "" && utf8.RuneCountInString(freeText) < 3 {
		return s.searchFallback(freeText, typeFilter, appFilter, pinnedOnly, limit, offset)
	}

	whereClauses := []string{}
	args := []interface{}{}

	if pinnedOnly {
		whereClauses = append(whereClauses, "i.pinned = 1")
	}
	if typeFilter != "" {
		whereClauses = append(whereClauses, "LOWER(i.type) = ?")
		args = append(args, typeFilter)
	}
	if appFilter != "" {
		whereClauses = append(whereClauses, `LOWER(i.source_app) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(appFilter)+"%")
	}

	var selectQuery string
	if freeText != "" {
		escapedFTS := strings.ReplaceAll(freeText, `"`, `""`)
		ftsMatch := `"` + escapedFTS + `"`
		whereClauses = append(whereClauses, "items_fts MATCH ?")
		args = append(args, ftsMatch)

		whereSQL := strings.Join(whereClauses, " AND ")
		selectQuery = fmt.Sprintf(`SELECT
			i.id, i.hash, i.type, i.mime, i.content, i.preview, i.rich_preview,
			i.blob_path, i.thumbnail_path, i.source_app, i.pinned, i.pinned_at,
			i.sensitive, i.masked_content, i.expires_at, i.size_bytes, i.created_at, i.updated_at
			FROM items i
			JOIN items_fts f ON i.rowid = f.rowid
			WHERE %s
			ORDER BY i.pinned DESC, rank, i.updated_at DESC
			LIMIT ? OFFSET ?`, whereSQL)
	} else {
		whereSQL := ""
		if len(whereClauses) > 0 {
			whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
		}
		selectQuery = fmt.Sprintf(`SELECT
			i.id, i.hash, i.type, i.mime, i.content, i.preview, i.rich_preview,
			i.blob_path, i.thumbnail_path, i.source_app, i.pinned, i.pinned_at,
			i.sensitive, i.masked_content, i.expires_at, i.size_bytes, i.created_at, i.updated_at
			FROM items i
			%s
			ORDER BY i.pinned DESC, i.updated_at DESC
			LIMIT ? OFFSET ?`, whereSQL)
	}

	var total int
	// For fast response, query up to limit + 1 rows
	selectArgs := append(args, limit+1, offset)
	rows, err := s.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return s.searchFallback(freeText, typeFilter, appFilter, pinnedOnly, limit, offset)
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, 0, err
	}

	total = offset + len(items)
	if len(items) > limit {
		items = items[:limit]
		total = offset + limit + 1 // indicates there is at least one more page
	}

	return items, total, nil
}

func (s *Store) searchFallback(text, typeFilter, appFilter string, pinnedOnly bool, limit, offset int) ([]Item, int, error) {
	whereClauses := []string{}
	args := []interface{}{}

	if text != "" {
		pat := "%" + escapeLike(text) + "%"
		whereClauses = append(whereClauses, `(i.content LIKE ? ESCAPE '\' OR i.preview LIKE ? ESCAPE '\')`)
		args = append(args, pat, pat)
	}
	if pinnedOnly {
		whereClauses = append(whereClauses, "i.pinned = 1")
	}
	if typeFilter != "" {
		whereClauses = append(whereClauses, "LOWER(i.type) = ?")
		args = append(args, typeFilter)
	}
	if appFilter != "" {
		whereClauses = append(whereClauses, `LOWER(i.source_app) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(appFilter)+"%")
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM items i %s`, whereSQL)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectQuery := fmt.Sprintf(`SELECT
		i.id, i.hash, i.type, i.mime, i.content, i.preview, i.rich_preview,
		i.blob_path, i.thumbnail_path, i.source_app, i.pinned, i.pinned_at,
		i.sensitive, i.masked_content, i.expires_at, i.size_bytes, i.created_at, i.updated_at
		FROM items i
		%s
		ORDER BY i.pinned DESC, i.updated_at DESC
		LIMIT ? OFFSET ?`, whereSQL)

	selectArgs := append(args, limit, offset)
	rows, err := s.db.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// isManagedItemPath returns true only if path is an absolute, lexically clean path
// pointing to a direct child within the store's blobsDir or thumbsDir,
// with no directory traversal components (".." or ".").
func (s *Store) isManagedItemPath(path string) bool {
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		return false
	}
	if filepath.Clean(path) != path {
		return false
	}
	dir := filepath.Dir(path)
	if dir != s.blobsDir && dir != s.thumbsDir {
		return false
	}
	base := filepath.Base(path)
	if base == "." || base == ".." || base == "" || strings.Contains(base, "/") || strings.Contains(base, "\\") {
		return false
	}
	return true
}

// SanitizeImportedItem neutralizes hostile or stale file references coming
// from an untrusted history export: blob/thumbnail paths are kept only when
// they live inside this store's managed dirs, contain no path traversal
// components, and the file exists as a regular file. Otherwise the row
// degrades to a plain item. An empty imported hash is derived from content
// so dedupe stays meaningful.
func (s *Store) SanitizeImportedItem(item *Item) {
	keep := func(path string) bool {
		if !s.isManagedItemPath(path) {
			return false
		}
		fi, err := os.Lstat(path)
		if err != nil || !fi.Mode().IsRegular() {
			return false
		}
		return true
	}
	if !keep(item.BlobPath) {
		item.BlobPath = ""
	}
	if !keep(item.ThumbnailPath) {
		item.ThumbnailPath = ""
	}
	if item.Type == "image" {
		item.Content = item.BlobPath
	}
	if item.Hash == "" {
		item.Hash = hashContent(item.Content)
	}
}

func (s *Store) GetItem(id string) (*Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT
		id, hash, type, mime, content, preview, rich_preview,
		blob_path, thumbnail_path, source_app, pinned, pinned_at,
		sensitive, masked_content, expires_at, size_bytes, created_at, updated_at
		FROM items WHERE id = ?`

	row := s.db.QueryRow(query, id)
	item, err := scanSingleItem(row)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Store) PinItem(id string, pinned bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pinnedAt int64
	if pinned {
		pinnedAt = time.Now().UnixMilli()
	}
	_, err := s.db.Exec("UPDATE items SET pinned = ?, pinned_at = ? WHERE id = ?", boolToInt(pinned), pinnedAt, id)
	return err
}

// LegacyRichPreviewItems returns code/json items whose stored rich preview
// predates the theme-agnostic class-based markup (identified by the absence
// of "t-" classes). The daemon re-renders these once at startup so old clips
// pick up theming without waiting for a re-copy.
func (s *Store) LegacyRichPreviewItems(limit int) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT id, type, content, preview, rich_preview, sensitive, created_at, updated_at
		 FROM items
		 WHERE type IN ('code', 'json')
		   AND rich_preview <> ''
		   AND rich_preview NOT LIKE '%class="t-%'
		 ORDER BY updated_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		var sensitiveInt int
		var createdAtMs, updatedAtMs int64
		if err := rows.Scan(&it.ID, &it.Type, &it.Content, &it.Preview, &it.RichPreview, &sensitiveInt, &createdAtMs, &updatedAtMs); err != nil {
			return nil, err
		}
		it.Sensitive = sensitiveInt == 1
		it.CreatedAt = time.UnixMilli(createdAtMs)
		it.UpdatedAt = time.UnixMilli(updatedAtMs)
		items = append(items, it)
	}
	return items, rows.Err()
}

// RefreshRichPreview rewrites only the rich_preview column. Markup format
// migrations must not touch updated_at: that would reshuffle the user's
// history order as a side effect of a rendering change.
func (s *Store) RefreshRichPreview(id, markup string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE items SET rich_preview = ? WHERE id = ?", markup, id)
	return err
}

func (s *Store) DeleteItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var blobPath, thumbPath string
	_ = s.db.QueryRow("SELECT blob_path, thumbnail_path FROM items WHERE id = ?", id).Scan(&blobPath, &thumbPath)

	_, err := s.db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Blobs are hash-named and may be referenced by several rows (imported
	// history, legacy state): only reap files whose last reference just went
	// away, and never touch paths outside the store's managed dirs.
	s.gcItemFile(blobPath)
	s.gcItemFile(thumbPath)

	return nil
}

// gcItemFile unlinks a store-managed item file only when no remaining row
// references it. Paths outside the blob/thumb dirs or containing traversal
// components are never unlinked — the row data may come from an imported
// or hand-edited database.
func (s *Store) gcItemFile(path string) {
	if !s.isManagedItemPath(path) {
		return
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return
	}
	var refs int
	if err := s.db.QueryRow(
		"SELECT COUNT(*) FROM items WHERE blob_path = ? OR thumbnail_path = ?", path, path,
	).Scan(&refs); err != nil || refs > 0 {
		return
	}
	_ = os.Remove(path)
}

// deleteRowsWithGC deletes rows matching a caller-built WHERE fragment over
// the items table (never user input), then reaps blob/thumb files whose last
// referencing row disappeared. Used by the bulk delete paths.
func (s *Store) deleteRowsWithGC(where string, args ...any) (int64, error) {
	rows, err := s.db.Query("SELECT DISTINCT blob_path, thumbnail_path FROM items WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	var paths []string
	for rows.Next() {
		var blobPath, thumbPath string
		if err := rows.Scan(&blobPath, &thumbPath); err != nil {
			rows.Close()
			return 0, err
		}
		paths = append(paths, blobPath, thumbPath)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	res, err := s.db.Exec("DELETE FROM items WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	for _, p := range paths {
		s.gcItemFile(p)
	}
	return res.RowsAffected()
}

func (s *Store) Clear(includePinned bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if includePinned {
		return s.deleteRowsWithGC("1 = 1")
	}
	return s.deleteRowsWithGC("pinned = 0")
}

func (s *Store) PurgeExpired() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	return s.deleteRowsWithGC("expires_at > 0 AND expires_at <= ?", now)
}

func (s *Store) EnforceRetention(maxItems int, maxAgeDays int, maxSizeBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Purge older than maxAgeDays (pinned exempt)
	if maxAgeDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -maxAgeDays).UnixMilli()
		_, _ = s.deleteRowsWithGC("pinned = 0 AND updated_at < ?", cutoff)
	}

	// 2. Limit count to maxItems (pinned exempt)
	if maxItems > 0 {
		var unpinnedCount int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM items WHERE pinned = 0").Scan(&unpinnedCount)
		if unpinnedCount > maxItems {
			excess := unpinnedCount - maxItems
			_, _ = s.deleteRowsWithGC(`id IN (
				SELECT id FROM items WHERE pinned = 0 ORDER BY updated_at ASC LIMIT ?
			)`, excess)
		}
	}

	// 3. Size cap
	if maxSizeBytes > 0 {
		var totalSize int64
		_ = s.db.QueryRow("SELECT COALESCE(SUM(size_bytes), 0) FROM items").Scan(&totalSize)
		for totalSize > maxSizeBytes {
			var oldestID string
			var itemSize int64
			err := s.db.QueryRow("SELECT id, size_bytes FROM items WHERE pinned = 0 ORDER BY updated_at ASC LIMIT 1").Scan(&oldestID, &itemSize)
			if err != nil {
				break // no more unpinned items
			}
			_, _ = s.deleteRowsWithGC("id = ?", oldestID)
			totalSize -= itemSize
		}
	}

	return nil
}

func (s *Store) GetStats() (total int, pinned int, sizeBytes int64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	err = s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(pinned), 0), COALESCE(SUM(size_bytes), 0) FROM items").Scan(&total, &pinned, &sizeBytes)
	return
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	var items []Item
	for rows.Next() {
		var it Item
		var pinnedInt, sensitiveInt int
		var createdAtMs, updatedAtMs int64

		err := rows.Scan(
			&it.ID, &it.Hash, &it.Type, &it.Mime, &it.Content, &it.Preview, &it.RichPreview,
			&it.BlobPath, &it.ThumbnailPath, &it.SourceApp, &pinnedInt, &it.PinnedAt,
			&sensitiveInt, &it.MaskedContent, &it.ExpiresAt, &it.SizeBytes,
			&createdAtMs, &updatedAtMs,
		)
		if err != nil {
			return nil, err
		}

		it.Pinned = pinnedInt == 1
		it.Sensitive = sensitiveInt == 1
		it.CreatedAt = time.UnixMilli(createdAtMs)
		it.UpdatedAt = time.UnixMilli(updatedAtMs)
		items = append(items, it)
	}
	return items, nil
}

func scanSingleItem(row *sql.Row) (*Item, error) {
	var it Item
	var pinnedInt, sensitiveInt int
	var createdAtMs, updatedAtMs int64

	err := row.Scan(
		&it.ID, &it.Hash, &it.Type, &it.Mime, &it.Content, &it.Preview, &it.RichPreview,
		&it.BlobPath, &it.ThumbnailPath, &it.SourceApp, &pinnedInt, &it.PinnedAt,
		&sensitiveInt, &it.MaskedContent, &it.ExpiresAt, &it.SizeBytes,
		&createdAtMs, &updatedAtMs,
	)
	if err != nil {
		return nil, err
	}

	it.Pinned = pinnedInt == 1
	it.Sensitive = sensitiveInt == 1
	it.CreatedAt = time.UnixMilli(createdAtMs)
	it.UpdatedAt = time.UnixMilli(updatedAtMs)
	return &it, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// hashContent is the canonical content hash: the hex encoding of the SHA-256
// digest of a content string (blobs are hashed through the same function so
// file naming and row hashes share one definition).
func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// escapeLike neutralizes SQL LIKE metacharacters in user input so that
// queries for "%" or "_" match those literal characters instead of acting
// as wildcards over the whole history.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
