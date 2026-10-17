package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newGCStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustSaveItem(t *testing.T, s *Store, item *Item) {
	t.Helper()
	if err := s.SaveItem(item); err != nil {
		t.Fatalf("SaveItem(%s): %v", item.ID, err)
	}
}

func imageItem(id string, blobPath string, size int64) *Item {
	return &Item{
		ID:        id,
		Hash:      sha256Hex(id + blobPath),
		Type:      "image",
		Mime:      "image/png",
		Content:   blobPath,
		Preview:   "Image",
		BlobPath:  blobPath,
		SizeBytes: size,
	}
}

// Two rows can point at the same hash-named blob (imported history, legacy
// state). Deleting one row must NOT unlink the file while another row still
// references it.
func TestDeleteItemKeepsSharedBlobAndThumb(t *testing.T) {
	s := newGCStore(t)

	blob, err := s.SaveBlob([]byte("fake png data"), "png")
	if err != nil {
		t.Fatalf("SaveBlob: %v", err)
	}
	thumb := filepath.Join(s.thumbsDir, "shared_thumb.png")
	if err := os.WriteFile(thumb, []byte("thumb"), 0o600); err != nil {
		t.Fatalf("write shared thumb: %v", err)
	}

	first := imageItem("row-1", blob, 14)
	second := imageItem("row-2", blob, 14)
	first.ThumbnailPath, second.ThumbnailPath = thumb, thumb
	mustSaveItem(t, s, first)
	mustSaveItem(t, s, second)

	if err := s.DeleteItem("row-1"); err != nil {
		t.Fatalf("DeleteItem row-1: %v", err)
	}
	if _, err := os.Stat(blob); err != nil {
		t.Fatalf("blob file was unlinked while row-2 still references it: %v", err)
	}
	if _, err := os.Stat(thumb); err != nil {
		t.Fatalf("thumb file was unlinked while row-2 still references it: %v", err)
	}

	if err := s.DeleteItem("row-2"); err != nil {
		t.Fatalf("DeleteItem row-2: %v", err)
	}
	if _, err := os.Stat(blob); !os.IsNotExist(err) {
		t.Fatalf("blob file must be reaped once the last referencing row is gone (err=%v)", err)
	}
	if _, err := os.Stat(thumb); !os.IsNotExist(err) {
		t.Fatalf("thumb file must be reaped once the last referencing row is gone (err=%v)", err)
	}
}

// Clear must reap blob files of deleted rows, but pinned rows keep theirs.
func TestClearReapsBlobFiles(t *testing.T) {
	s := newGCStore(t)

	blobA, err := s.SaveBlob([]byte("image A"), "png")
	if err != nil {
		t.Fatalf("SaveBlob A: %v", err)
	}
	blobB, err := s.SaveBlob([]byte("image B png"), "png")
	if err != nil {
		t.Fatalf("SaveBlob B: %v", err)
	}

	unpinned := imageItem("row-a", blobA, 7)
	pinned := imageItem("row-b", blobB, 10)
	pinned.Pinned, pinned.PinnedAt = true, time.Now().UnixMilli()
	mustSaveItem(t, s, unpinned)
	mustSaveItem(t, s, pinned)

	// Clear without pinned: row-a gone, its blob reaped; row-b (and blob) stays.
	if n, err := s.Clear(false); err != nil || n != 1 {
		t.Fatalf("Clear(false): got n=%d err=%v", n, err)
	}
	if _, err := os.Stat(blobA); !os.IsNotExist(err) {
		t.Fatalf("cleared row's blob must be reaped (err=%v)", err)
	}
	if _, err := os.Stat(blobB); err != nil {
		t.Fatalf("pinned row's blob must survive a non-pinned clear: %v", err)
	}

	// Full clear: pinned row goes too, blob gets reaped.
	if n, err := s.Clear(true); err != nil || n != 1 {
		t.Fatalf("Clear(true): got n=%d err=%v", n, err)
	}
	if _, err := os.Stat(blobB); !os.IsNotExist(err) {
		t.Fatalf("fully cleared row's blob must be reaped (err=%v)", err)
	}
}

func TestPurgeExpiredReapsBlobFiles(t *testing.T) {
	s := newGCStore(t)

	blob, err := s.SaveBlob([]byte("sensitive image"), "png")
	if err != nil {
		t.Fatalf("SaveBlob: %v", err)
	}
	item := imageItem("expired", blob, 15)
	item.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	mustSaveItem(t, s, item)

	if n, err := s.PurgeExpired(); err != nil || n != 1 {
		t.Fatalf("PurgeExpired: n=%d err=%v", n, err)
	}
	if _, err := os.Stat(blob); !os.IsNotExist(err) {
		t.Fatalf("purged item's blob must be reaped (err=%v)", err)
	}
}

func TestEnforceRetentionReapsBlobFiles(t *testing.T) {
	s := newGCStore(t)

	blobOld, err := s.SaveBlob([]byte("old image"), "png")
	if err != nil {
		t.Fatalf("SaveBlob old: %v", err)
	}
	blobNew, err := s.SaveBlob([]byte("new image data"), "png")
	if err != nil {
		t.Fatalf("SaveBlob new: %v", err)
	}

	old := imageItem("row-old", blobOld, 9)
	newer := imageItem("row-new", blobNew, 14)
	// updated_at is stamped by SaveItem; save in age order with a real gap so
	// retention deterministically picks row-old as the excess item.
	mustSaveItem(t, s, old)
	time.Sleep(5 * time.Millisecond)
	mustSaveItem(t, s, newer)

	if err := s.EnforceRetention(1, 0, 0); err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}

	if _, err := os.Stat(blobOld); !os.IsNotExist(err) {
		t.Fatalf("retention-deleted item's blob must be reaped (err=%v)", err)
	}
	if _, err := os.Stat(blobNew); err != nil {
		t.Fatalf("retained item's blob must survive: %v", err)
	}
}

// Blob paths outside the store's own data dirs (e.g. inherited from a hand
// edited or imported database) must never be unlinked from the filesystem.
func TestDeleteItemNeverUnlinksForeignPaths(t *testing.T) {
	s := newGCStore(t)

	outsideDir := t.TempDir() // intentionally different from the store's dir
	decoy := filepath.Join(outsideDir, "decoy.png")
	if err := os.WriteFile(decoy, []byte("do not delete me"), 0o600); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	item := imageItem("imported", decoy, 15)
	mustSaveItem(t, s, item)

	if err := s.DeleteItem("imported"); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if _, err := os.Stat(decoy); err != nil {
		t.Fatalf("foreign blob path was unlinked by DeleteItem: %v", err)
	}
}

// FTS5 trigram needs >= 3 characters; shorter queries must fall back to the
// LIKE path so "he" and Persian 2-char prefixes still match.
func TestSearchShortQueryStillMatches(t *testing.T) {
	s := newGCStore(t)

	hello := &Item{Hash: sha256Hex("hello world"), Type: "text", Mime: "text/plain", Content: "hello world", Preview: "hello world", SizeBytes: 11}
	persian := &Item{Hash: sha256Hex("سلام دنیا"), Type: "text", Mime: "text/plain", Content: "سلام دنیا", Preview: "سلام دنیا", SizeBytes: 18}
	mustSaveItem(t, s, hello)
	mustSaveItem(t, s, persian)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"3-rune prefix keeps working", "hel", "hello world"},
		{"2-rune ASCII prefix", "he", "hello world"},
		{"1-rune ASCII prefix", "h", "hello world"},
		{"2-rune Persian prefix", "سل", "سلام دنیا"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, _, err := s.SearchItems(tc.query, 10, 0)
			if err != nil {
				t.Fatalf("SearchItems(%q): %v", tc.query, err)
			}
			if len(items) != 1 || items[0].Content != tc.want {
				got := "none"
				if len(items) > 0 {
					got = fmt.Sprintf("want %q, got %q", tc.want, items[0].Content)
				}
				t.Fatalf("short query %q must return exactly one row; %s", tc.query, got)
			}
		})
	}
}
