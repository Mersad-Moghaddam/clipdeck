package store

import (
	"strings"
	"testing"
)

func TestQueryRecentOrderUsesIndex(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.Query("EXPLAIN QUERY PLAN SELECT id FROM items ORDER BY pinned DESC,updated_at DESC,id DESC LIMIT 51")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "TEMP B-TREE") {
			t.Fatalf("unnecessary sort: %s", detail)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
