package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

// writeWlPasteStub installs a custom wl-paste stub on PATH and wires the
// environment the way capture tests expect (isolated HOME / XDG_RUNTIME_DIR).
func writeWlPasteStub(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wl-paste")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub wl-paste: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	if _, err := exec.LookPath("wl-paste"); err != nil {
		t.Fatalf("stub wl-paste not on PATH: %v", err)
	}
}

func newTestCapturer(t *testing.T, onEvent func(string, *store.Item)) (*Capturer, *store.Store) {
	t.Helper()
	// Isolate every test from the real user store, even when no wl-paste
	// stub is installed.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cfg := config.DefaultConfig("")
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(cfg, st, onEvent), st
}

// The stub advertises a uri-list selection; any payload query returns one
// file URI (like wl-paste --type text/uri-list does for file copies).
const uriListStub = `#!/bin/sh
for arg in "$@"; do
	case "$arg" in
	--list-types)
		printf 'text/uri-list\n'
		exit 0
		;;
	--watch)
		printf '\n'
		exec sleep 3
		;;
	esac
done
printf '%s\n' 'file:///home/user/report.pdf'
exit 0
`

// Pausing must also gate the inline uri-list branch of SnapshotCurrent — it
// is the one copy path that builds and saves an item outside the
// ProcessTextContent / ProcessImageContent guards.
func TestSnapshotUriListHonorsPause(t *testing.T) {
	writeWlPasteStub(t, uriListStub)
	cap, st := newTestCapturer(t, nil)

	cap.SetPaused(true)
	item, err := cap.SnapshotCurrent()
	if err != nil {
		t.Fatalf("snapshot while paused: %v", err)
	}
	if item != nil {
		t.Fatalf("uri-list was captured while paused")
	}
	_, total, err := st.ListItems(10, 0)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if total != 0 {
		t.Fatalf("store changed while paused: %d rows", total)
	}

	// Unpaused: the same snapshot must capture the file copy.
	cap.SetPaused(false)
	item, err = cap.SnapshotCurrent()
	if err != nil {
		t.Fatalf("snapshot while running: %v", err)
	}
	if item == nil || item.Type != "file" {
		t.Fatalf("expected file item when running, got %+v", item)
	}
}

// Sensitive clips must not carry raw (unmasked) content in their rich
// preview; the reveal only ever happens on explicit user action.
func TestSensitiveItemHasNoRichPreview(t *testing.T) {
	cap, _ := newTestCapturer(t, nil)

	// A private-key block is detected as sensitive AND shaped like code, so a
	// naive rich render would ship the raw material as markup.
	secret := "-----BEGIN RSA PRIVATE KEY-----\n" + strings.Repeat("QWJD", 16) + "\n-----END RSA PRIVATE KEY-----"
	item, err := cap.ProcessTextContent(secret, []string{"text/plain"}, "kitty")
	if err != nil {
		t.Fatalf("ProcessTextContent: %v", err)
	}
	if item == nil {
		t.Fatal("expected text item")
	}
	if !item.Sensitive {
		t.Fatalf("private key not detected as sensitive")
	}
	if item.RichPreview != "" {
		t.Fatalf("sensitive item rich_preview leaks raw content: %q", item.RichPreview)
	}
	if item.Content != secret {
		t.Fatalf("content must stay intact for explicit reveal")
	}
}

// The text and image watchers each snapshot the same selection change; the
// second ingest of the SAME content within the debounce window must not
// re-save + re-broadcast. Distinct content must always process.
func TestDuplicateEchoOfSameContentIsSuppressed(t *testing.T) {
	spy := newEventSpy()
	cap, st := newTestCapturer(t, spy.record)

	first, err := cap.ProcessTextContent("hello echo world", []string{"text/plain"}, "kitty")
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first == nil {
		t.Fatal("first ingest dropped")
	}

	second, err := cap.ProcessTextContent("hello echo world", []string{"text/plain"}, "kitty")
	if err != nil {
		t.Fatalf("echo ingest: %v", err)
	}
	if second != nil {
		t.Fatal("same-content echo within the debounce window must not re-ingest")
	}
	if got := spy.count("item_added"); got != 1 {
		t.Fatalf("item_added broadcast count = %d, want 1", got)
	}
	items, _, err := st.ListItems(10, 0)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("echo created a duplicate row: %d rows", len(items))
	}

	// Genuinely new content in the same window must still be captured.
	third, err := cap.ProcessTextContent("different payload", []string{"text/plain"}, "kitty")
	if err != nil {
		t.Fatalf("distinct ingest: %v", err)
	}
	if third == nil {
		t.Fatal("distinct content was suppressed together with the echo")
	}
	if got := spy.count("item_added"); got != 2 {
		t.Fatalf("item_added broadcast count = %d, want 2", got)
	}
}

type eventSpy struct {
	mu     chan struct{}
	events []string
}

func newEventSpy() *eventSpy {
	return &eventSpy{mu: make(chan struct{}, 1)}
}

func (s *eventSpy) record(event string, _ *store.Item) {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()
	s.events = append(s.events, event)
}

func (s *eventSpy) count(event string) int {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()
	n := 0
	for _, e := range s.events {
		if e == event {
			n++
		}
	}
	return n
}

// Blob extensions must follow the selection's mime type; gif/bmp salvaged
// clipboard frames must not be stored as .png.
func TestImageExtensionFollowsMime(t *testing.T) {
	cap, _ := newTestCapturer(t, nil)

	cases := map[string]string{
		"image/gif":  ".gif",
		"image/bmp":  ".bmp",
		"image/jpeg": ".jpg",
		"image/webp": ".webp",
		"image/png":  ".png",
	}
	for mime, want := range cases {
		t.Run(mime, func(t *testing.T) {
			data := []byte(fmt.Sprintf("fake %s data", mime))
			item, err := cap.ProcessImageContent(data, mime, "grim")
			if err != nil {
				t.Fatalf("ProcessImageContent(%s): %v", mime, err)
			}
			if item == nil {
				t.Fatalf("%s ingest dropped", mime)
			}
			if !strings.HasSuffix(item.BlobPath, want) {
				t.Fatalf("mime %s must produce suffix %q, got %q", mime, want, item.BlobPath)
			}
		})
	}
}

// An empty selection (wl-paste exits 1 on --list-types) is a normal
// transition, not an error; SnapshotCurrent must return silently so the
// watcher loop does not log noise per event.
func TestSnapshotEmptySelectionIsSilent(t *testing.T) {
	emptyStub := `#!/bin/sh
for arg in "$@"; do
	case "$arg" in
	--list-types)
		exit 1
		;;
	esac
done
exit 1
`
	writeWlPasteStub(t, emptyStub)
	cap, _ := newTestCapturer(t, nil)

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = cap.SnapshotCurrent()
	}()
	select {
	case <-done:
		if err != nil {
			t.Fatalf("empty selection must be silent (got err=%v)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SnapshotCurrent blocked on an empty selection")
	}
}
