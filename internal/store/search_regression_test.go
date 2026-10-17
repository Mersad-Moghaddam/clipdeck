package store

import (
	"fmt"
	"testing"
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
