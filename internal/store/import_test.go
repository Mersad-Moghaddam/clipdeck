package store

import (
	"path/filepath"
	"testing"
)

// History exports are untrusted input: blob/thumbnail paths must either be
// inside the store's own managed dirs AND actually exist, or be dropped, so
// that a later DeleteItem/Clear can never unlink arbitrary files.
func TestSanitizeImportedItem(t *testing.T) {
	s := newGCStore(t)

	t.Run("foreign paths are dropped", func(t *testing.T) {
		item := &Item{
			Hash:          "xyz",
			Type:          "image",
			Content:       "/etc/passwd",
			BlobPath:      "/etc/passwd",
			ThumbnailPath: "/boot/vmlinuz",
		}
		s.SanitizeImportedItem(item)
		if item.BlobPath != "" || item.ThumbnailPath != "" {
			t.Fatalf("foreign paths survived sanitization: blob=%q thumb=%q", item.BlobPath, item.ThumbnailPath)
		}
	})

	t.Run("managed and present blob survives round-trip", func(t *testing.T) {
		blob, err := s.SaveBlob([]byte("roundtrip"), "png")
		if err != nil {
			t.Fatalf("SaveBlob: %v", err)
		}
		item := &Item{Hash: "abc", Type: "image", Content: blob, BlobPath: blob}
		s.SanitizeImportedItem(item)
		if item.BlobPath != blob {
			t.Fatalf("valid managed blob was dropped: %q", item.BlobPath)
		}
	})

	t.Run("managed but missing blob is dropped", func(t *testing.T) {
		ghost := filepath.Join(s.blobsDir, "ghost.png")
		item := &Item{Hash: "def", Type: "image", Content: ghost, BlobPath: ghost}
		s.SanitizeImportedItem(item)
		if item.BlobPath != "" {
			t.Fatalf("stale managed blob survived: %q", item.BlobPath)
		}
	})

	t.Run("empty hash is derived from content", func(t *testing.T) {
		item := &Item{Type: "text", Mime: "text/plain", Content: "hello hash"}
		s.SanitizeImportedItem(item)
		if item.Hash == "" {
			t.Fatal("SanitizeImportedItem must derive a hash from content")
		}
		if item.Hash != sha256Hex("hello hash") {
			t.Fatalf("hash mismatch: %q", item.Hash)
		}
	})
}
