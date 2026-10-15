package store

import (
	"os"
	"testing"
)

func TestLegacyRichPreviewRefresh(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clipdeck-store-markup-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	legacy := &Item{
		Hash:        sha256Hex("legacy-code"),
		Type:        "code",
		Mime:        "text/plain",
		Content:     "package main\n",
		Preview:     "package main",
		RichPreview: `<pre style="background-color:#282a36"><code>1 package main</code></pre>`,
	}
	fresh := &Item{
		Hash:        sha256Hex("fresh-code"),
		Type:        "code",
		Mime:        "text/plain",
		Content:     "package fresh\n",
		Preview:     "package fresh",
		RichPreview: `<span class="t-ln">1</span><span class="t-keyword">package</span>`,
	}
	plain := &Item{
		Hash:        sha256Hex("plain-text"),
		Type:        "text",
		Mime:        "text/plain",
		Content:     "hello",
		Preview:     "hello",
		RichPreview: "<pre>ignored because not code</pre>",
	}

	for _, item := range []*Item{legacy, fresh, plain} {
		if err := s.SaveItem(item); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}
	}

	items, err := s.LegacyRichPreviewItems(10)
	if err != nil {
		t.Fatalf("LegacyRichPreviewItems: %v", err)
	}
	if len(items) != 1 || items[0].ID != legacy.ID {
		t.Fatalf("expected only the legacy code item, got %d items", len(items))
	}

	if err := s.RefreshRichPreview(legacy.ID, `<span class="t-ln">1</span>`); err != nil {
		t.Fatalf("RefreshRichPreview: %v", err)
	}

	remaining, err := s.LegacyRichPreviewItems(10)
	if err != nil {
		t.Fatalf("LegacyRichPreviewItems after refresh: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no legacy items left, got %d", len(remaining))
	}

	got, err := s.GetItem(legacy.ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got.RichPreview != `<span class="t-ln">1</span>` {
		t.Fatalf("rich preview not refreshed: %q", got.RichPreview)
	}
	// RefreshRichPreview must not disturb the history ordering fields.
	if got.UpdatedAt.UnixMilli() != legacy.UpdatedAt.UnixMilli() {
		t.Fatalf("refresh changed updated_at: %v != %v", got.UpdatedAt, legacy.UpdatedAt)
	}
}
