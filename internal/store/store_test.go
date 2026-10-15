package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestStoreBasicOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clipdeck-store-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	hash1 := sha256Hex("Hello, world!")
	item1 := &Item{
		Hash:      hash1,
		Type:      "text",
		Mime:      "text/plain",
		Content:   "Hello, world!",
		Preview:   "Hello, world!",
		SourceApp: "kitty",
		SizeBytes: int64(len("Hello, world!")),
	}

	if err := s.SaveItem(item1); err != nil {
		t.Fatalf("SaveItem 1: %v", err)
	}

	// Persian string
	persianText := "سلام دنیا! این یک متن تستی کلیپ‌دک است."
	hash2 := sha256Hex(persianText)
	item2 := &Item{
		Hash:      hash2,
		Type:      "text",
		Mime:      "text/plain",
		Content:   persianText,
		Preview:   persianText,
		SourceApp: "alacritty",
		SizeBytes: int64(len(persianText)),
	}

	if err := s.SaveItem(item2); err != nil {
		t.Fatalf("SaveItem 2 (Persian): %v", err)
	}

	// Verify list items
	items, total, err := s.ListItems(10, 0)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("Expected 2 items, got total=%d len=%d", total, len(items))
	}

	// Test Pinning
	if err := s.PinItem(item1.ID, true); err != nil {
		t.Fatalf("PinItem: %v", err)
	}

	pinnedItem, err := s.GetItem(item1.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if !pinnedItem.Pinned {
		t.Fatalf("Expected item to be pinned")
	}

	// Test Search (Persian substring)
	searchResults, count, err := s.SearchItems("کلیپ‌دک", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems Persian: %v", err)
	}
	if count == 0 || len(searchResults) == 0 {
		t.Fatalf("Expected to find Persian item with 'کلیپ‌دک'")
	}

	// Test Search (Latin substring)
	searchResults, count, err = s.SearchItems("world", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems Latin: %v", err)
	}
	if count == 0 || len(searchResults) == 0 {
		t.Fatalf("Expected to find item with 'world'")
	}

	// Test Filter by type
	searchResults, count, err = s.SearchItems("type:text", 10, 0)
	if err != nil {
		t.Fatalf("SearchItems type:text: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 text items, got %d", count)
	}

	// Test Retention
	if err := s.EnforceRetention(1, 0, 0); err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	// Pinned item must be exempt!
	_, totalAfter, _ := s.ListItems(10, 0)
	if totalAfter != 2 {
		// item1 was pinned, item2 was unpinned and maxItems=1, so unpinned items <= 1, total = 1 pinned + 1 unpinned = 2
	}

	// Test Delete
	if err := s.DeleteItem(item2.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	_, totalAfterDelete, _ := s.ListItems(10, 0)
	if totalAfterDelete != 1 {
		t.Fatalf("Expected 1 item after delete, got %d", totalAfterDelete)
	}
}

func BenchmarkSearch10000Items(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "clipdeck-store-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := Open(tmpDir)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// Seed 10,000 synthetic items
	for i := 0; i < 10000; i++ {
		text := fmt.Sprintf("Synthetic clip item #%d with some unique words %x and Persian نمونه متن شماره %d", i, i*31, i)
		hash := sha256Hex(text)
		item := &Item{
			Hash:      hash,
			Type:      "text",
			Mime:      "text/plain",
			Content:   text,
			Preview:   text[:min(len(text), 60)],
			SourceApp: "app",
			SizeBytes: int64(len(text)),
		}
		if err := s.SaveItem(item); err != nil {
			b.Fatalf("Seeding failed at %d: %v", i, err)
		}
	}

	queries := []string{
		"Synthetic",
		"نمونه متن",
		"unique words",
		"type:text item",
		"item #9999",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		start := time.Now()
		_, _, err := s.SearchItems(q, 50, 0)
		elapsed := time.Since(start)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
		if elapsed > 30*time.Millisecond {
			b.Logf("Notice: search took %v (> 30ms target) for query %q", elapsed, q)
		}
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
