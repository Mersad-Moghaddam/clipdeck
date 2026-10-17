package store

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// QueryOptions applies explicit filters before pagination. Legacy list/search
// retain their existing total-count semantics.
type QueryOptions struct {
	Query      string   `json:"query"`
	Types      []string `json:"types"`
	PinnedOnly bool     `json:"pinned_only"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
}
type QueryPage struct {
	Items   []Item `json:"items"`
	HasMore bool   `json:"has_more"`
}

func (s *Store) QueryItems(opts QueryOptions) (QueryPage, error) {
	page := QueryPage{Items: []Item{}}
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	if opts.Limit < 1 || opts.Limit > 200 || opts.Offset < 0 {
		return page, fmt.Errorf("limit must be 1..200 and offset nonnegative")
	}
	allowed := map[string]bool{"text": true, "code": true, "json": true, "link": true, "color": true, "image": true, "email": true, "phone": true, "number": true, "file": true, "path": true}
	clauses := []string{}
	args := []any{}
	if len(opts.Types) > 0 {
		marks := []string{}
		for _, kind := range opts.Types {
			if !allowed[kind] {
				return page, fmt.Errorf("unknown type %q", kind)
			}
			marks = append(marks, "?")
			args = append(args, kind)
		}
		clauses = append(clauses, "i.type IN ("+strings.Join(marks, ",")+")")
	}
	terms := []string{}
	for _, part := range strings.Fields(opts.Query) {
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, "type:"):
			clauses = append(clauses, "LOWER(i.type) = ?")
			args = append(args, lower[5:])
		case strings.HasPrefix(lower, "app:"):
			clauses = append(clauses, `LOWER(i.source_app) LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLike(lower[4:])+"%")
		case lower == "pinned" || lower == "pinned:true":
			opts.PinnedOnly = true
		default:
			terms = append(terms, part)
		}
	}
	if opts.PinnedOnly {
		clauses = append(clauses, "i.pinned = 1")
	}
	text := strings.Join(terms, " ")
	join := ""
	order := "i.pinned DESC, i.updated_at DESC, i.id DESC"
	if utf8.RuneCountInString(text) >= 3 {
		join = " JOIN items_fts ON i.rowid = items_fts.rowid"
		clauses = append(clauses, "items_fts MATCH ?")
		args = append(args, `"`+strings.ReplaceAll(text, `"`, `""`)+`"`)
		order = "i.pinned DESC, rank, i.updated_at DESC, i.id DESC"
	} else if text != "" {
		clauses = append(clauses, `(i.content LIKE ? ESCAPE '\' OR i.preview LIKE ? ESCAPE '\')`)
		pat := "%" + escapeLike(text) + "%"
		args = append(args, pat, pat)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	query := `SELECT i.id,i.hash,i.type,i.mime,i.content,i.preview,i.rich_preview,i.blob_path,i.thumbnail_path,i.source_app,i.pinned,i.pinned_at,i.sensitive,i.masked_content,i.expires_at,i.size_bytes,i.created_at,i.updated_at FROM items i` + join + where + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args = append(args, opts.Limit+1, opts.Offset)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	items, err := scanItems(rows)
	if err != nil {
		return page, err
	}
	page.HasMore = len(items) > opts.Limit
	if page.HasMore {
		items = items[:opts.Limit]
	}
	if len(items) > 0 {
		page.Items = items
	}
	return page, nil
}
