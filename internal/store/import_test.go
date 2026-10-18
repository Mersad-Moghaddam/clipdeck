package store

import (
	"os"
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

	t.Run("traversal paths are dropped", func(t *testing.T) {
		outsideFile := filepath.Join(t.TempDir(), "victim.txt")
		if err := os.WriteFile(outsideFile, []byte("sensitive"), 0600); err != nil {
			t.Fatalf("write outside file: %v", err)
		}
		rel, err := filepath.Rel(s.blobsDir, outsideFile)
		if err != nil {
			t.Fatalf("rel path: %v", err)
		}
		traversalPath := s.blobsDir + "/" + rel

		traversalCases := []string{
			traversalPath,
			s.blobsDir + "/../../victim.txt",
			s.blobsDir + "/../blobs/victim.txt",
			s.blobsDir + "/./victim.txt",
			s.blobsDir + "//victim.txt",
			"relative/path/to/victim.txt",
			"../victim.txt",
			"",
			s.blobsDir,
			s.thumbsDir,
		}

		for _, tc := range traversalCases {
			item := &Item{
				Hash:          "traversal",
				Type:          "image",
				Content:       tc,
				BlobPath:      tc,
				ThumbnailPath: tc,
			}
			s.SanitizeImportedItem(item)
			if item.BlobPath != "" {
				t.Errorf("BlobPath %q survived sanitization", tc)
			}
			if item.ThumbnailPath != "" {
				t.Errorf("ThumbnailPath %q survived sanitization", tc)
			}
			if item.Content != "" {
				t.Errorf("image Content %q was not neutralized when BlobPath was invalid", item.Content)
			}
		}
	})

	t.Run("symlinks pointing outside are dropped", func(t *testing.T) {
		victimFile := filepath.Join(t.TempDir(), "secret.txt")
		if err := os.WriteFile(victimFile, []byte("secret"), 0600); err != nil {
			t.Fatalf("write victim file: %v", err)
		}
		symlinkBlob := filepath.Join(s.blobsDir, "symlink.png")
		if err := os.Symlink(victimFile, symlinkBlob); err != nil {
			t.Fatalf("create symlink: %v", err)
		}
		defer os.Remove(symlinkBlob)

		item := &Item{
			Hash:          "symlink",
			Type:          "image",
			Content:       symlinkBlob,
			BlobPath:      symlinkBlob,
			ThumbnailPath: symlinkBlob,
		}
		s.SanitizeImportedItem(item)
		if item.BlobPath != "" || item.ThumbnailPath != "" {
			t.Fatalf("symlink survived sanitization: blob=%q thumb=%q", item.BlobPath, item.ThumbnailPath)
		}
	})

	t.Run("subdirectories in blobsDir are dropped", func(t *testing.T) {
		subDir := filepath.Join(s.blobsDir, "nested")
		if err := os.MkdirAll(subDir, 0700); err != nil {
			t.Fatalf("mkdir subDir: %v", err)
		}
		defer os.RemoveAll(subDir)

		item := &Item{
			Hash:          "nested_dir",
			Type:          "image",
			BlobPath:      subDir,
			ThumbnailPath: subDir,
		}
		s.SanitizeImportedItem(item)
		if item.BlobPath != "" || item.ThumbnailPath != "" {
			t.Fatalf("directory path survived sanitization: blob=%q thumb=%q", item.BlobPath, item.ThumbnailPath)
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
