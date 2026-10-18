package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestImportImageContentUsesSanitizedBlob(t *testing.T) {
	s := newGCStore(t)
	victim := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(victim, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.blobsDir, "imported.png")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, filepath.Join(s.blobsDir, "missing.png")} {
		item := &Item{Type: "image", Content: path, BlobPath: path, ThumbnailPath: path}
		s.SanitizeImportedItem(item)
		if item.Content != "" || item.BlobPath != "" || item.ThumbnailPath != "" {
			t.Errorf("rejected image reference retained: %+v", item)
		}
	}
}

func TestSaveBlobRejectsExistingSymlink(t *testing.T) {
	s := newGCStore(t)
	data := []byte("image")
	victim := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(victim, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.blobsDir, hashContent(string(data))+".png")
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBlob(data, "png"); err == nil {
		t.Fatal("accepted existing symlink as a blob")
	}
}

func TestSaveBlobRejectsUnsafeExtensions(t *testing.T) {
	s := newGCStore(t)
	for _, ext := range []string{"/../../outside", "png/../../outside", `png\outside`, "png\x00"} {
		if path, err := s.SaveBlob([]byte("image"), ext); err == nil {
			t.Errorf("accepted extension %q: %q", ext, path)
		}
	}
}

func TestSaveBlobConcurrentWrites(t *testing.T) {
	s := newGCStore(t)
	data := []byte("complete image bytes")
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			path, err := s.SaveBlob(data, "png")
			if err != nil {
				t.Errorf("concurrent save: %v", err)
				return
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(data) {
				t.Errorf("incomplete blob: %q, %v", got, err)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestSaveBlobDoesNotFollowTemporarySymlink(t *testing.T) {
	s := newGCStore(t)
	data := []byte("new image")
	victim := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(victim, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	legacyTemp := filepath.Join(s.blobsDir, hashContent(string(data))+".png.tmp")
	if err := os.Symlink(victim, legacyTemp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBlob(data, "png"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep me" {
		t.Fatalf("SaveBlob overwrote outside file: %q", got)
	}
}
