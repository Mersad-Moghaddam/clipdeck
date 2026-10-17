package store

import (
	"fmt"
	"testing"
	"time"
)

func TestSearchFilterCase(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	item := &Item{Hash: "case", Type: "text", Mime: "text/plain", Content: "needle", Preview: "needle", SourceApp: "Firefox", SizeBytes: 6}
	if err := s.SaveItem(item); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"TYPE:text needle", "APP:firefox needle"} {
		items, _, err := s.SearchItems(q, 50, 0)
		if err != nil || len(items) != 1 {
			t.Errorf("%q: got %d items, %v", q, len(items), err)
		}
	}
}

func TestQueryFiltersBeforeLimit(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 205; i++ {
		kind := "text"
		if i == 0 {
			kind = "code"
		}
		item := &Item{Hash: fmt.Sprint(i), Type: kind, Mime: "text/plain", Content: "matching clip", Preview: "matching clip", SizeBytes: 13}
		if err := s.SaveItem(item); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.QueryItems(QueryOptions{Types: []string{"code"}, Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].Type != "code" || page.HasMore {
		t.Fatalf("unexpected page: %+v %v", page, err)
	}
	page, err = s.QueryItems(QueryOptions{Types: []string{"text"}, Limit: 2})
	if err != nil || len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("unexpected text page: %+v %v", page, err)
	}
}

func TestQueryValidation(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, opts := range []QueryOptions{{Limit: 201}, {Offset: -1}, {Types: []string{"invalid"}}} {
		if _, err := s.QueryItems(opts); err == nil {
			t.Errorf("accepted %+v", opts)
		}
	}
}

func TestQueryStablePaging(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 5; i++ {
		item := &Item{Hash: fmt.Sprint(i), Type: "text", Mime: "text/plain", Content: "needle", Preview: "needle", SizeBytes: 6}
		if err := s.SaveItem(item); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for offset := 0; offset < 5; offset += 2 {
		page, err := s.QueryItems(QueryOptions{Query: "needle", Limit: 2, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("duplicate page ID")
			}
			seen[item.ID] = true
		}
		if page.HasMore != (offset < 4) {
			t.Fatalf("has_more at offset %d", offset)
		}
	}
	if len(seen) != 5 {
		t.Fatal("lost clips")
	}
}

func TestSearchEmptyConcurrentWriter(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 5; i++ {
		item := &Item{Hash: fmt.Sprint(i), Type: "text", Mime: "text/plain", Content: fmt.Sprintf("clip %d", i), Preview: fmt.Sprintf("clip %d", i), SizeBytes: 6}
		if err := s.SaveItem(item); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			// Search with empty query should delegate to ListItems without deadlocking on s.mu
			items, _, err := s.SearchItems("", 10, 0)
			if err != nil || len(items) == 0 {
				t.Errorf("search empty error: %v", err)
			}
		}
	}()

	// Concurrent writes
	for i := 10; i < 20; i++ {
		item := &Item{Hash: fmt.Sprint(i), Type: "text", Mime: "text/plain", Content: fmt.Sprintf("clip %d", i), Preview: fmt.Sprintf("clip %d", i), SizeBytes: 6}
		_ = s.SaveItem(item)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TestSearchEmptyConcurrentWriter timed out: possible nested-lock deadlock")
	}
}

func TestSearchLiteralEscapes(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	items := []*Item{
		{Hash: "pct", Type: "text", Mime: "text/plain", Content: "50% off everything", Preview: "50% off everything", SizeBytes: 18},
		{Hash: "und", Type: "text", Mime: "text/plain", Content: "my_variable_name", Preview: "my_variable_name", SizeBytes: 16},
		{Hash: "bs", Type: "text", Mime: "text/plain", Content: `C:\Program Files\App`, Preview: `C:\Program Files\App`, SizeBytes: 20},
		{Hash: "qt", Type: "text", Mime: "text/plain", Content: `He said "Hello world"`, Preview: `He said "Hello world"`, SizeBytes: 21},
	}
	for _, it := range items {
		if err := s.SaveItem(it); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		q    string
		want int
	}{
		{"50%", 1},
		{"my_variable", 1},
		{`Program Files`, 1},
		{`"Hello world"`, 1},
	}

	for _, tc := range tests {
		res, _, err := s.SearchItems(tc.q, 10, 0)
		if err != nil {
			t.Errorf("query %q: %v", tc.q, err)
		} else if len(res) != tc.want {
			t.Errorf("query %q: got %d items, want %d", tc.q, len(res), tc.want)
		}
	}
}

func TestSearchUnicodeAndShortTerms(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	items := []*Item{
		{Hash: "fa1", Type: "text", Mime: "text/plain", Content: "سلام دنیا", Preview: "سلام دنیا", SizeBytes: 16},
		{Hash: "fa2", Type: "text", Mime: "text/plain", Content: "کد پستی", Preview: "کد پستی", SizeBytes: 14},
		{Hash: "en1", Type: "text", Mime: "text/plain", Content: "Go language", Preview: "Go language", SizeBytes: 11},
	}
	for _, it := range items {
		if err := s.SaveItem(it); err != nil {
			t.Fatal(err)
		}
	}

	// 1 or 2 runes use fallback LIKE
	shortQueries := []string{"کد", "Go", "سل"}
	for _, q := range shortQueries {
		res, _, err := s.SearchItems(q, 10, 0)
		if err != nil {
			t.Errorf("short query %q: %v", q, err)
		} else if len(res) == 0 {
			t.Errorf("short query %q: got 0 results, expected matches", q)
		}
	}
}

func TestSearchUpgradeReopen(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	item := &Item{Hash: "persist", Type: "code", Mime: "text/plain", Content: "package main\nfunc main() {}", Preview: "package main", SizeBytes: 25}
	if err := s1.SaveItem(item); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	res, _, err := s2.SearchItems("package main", 10, 0)
	if err != nil || len(res) != 1 {
		t.Fatalf("reopened store search failed: %v, got %d items", err, len(res))
	}
}
