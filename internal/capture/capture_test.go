package capture

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

// stubWlPasteScript emulates the wl-clipboard surface ClipDeck uses so the
// watcher -> snapshot -> store pipeline can be exercised without a Wayland
// session:
//
//	--watch        emit one change event, then stay connected (like wl-paste)
//	--list-types   advertise a text selection
//	--no-newline   return the current selection payload
//	(anything else) exit silently, e.g. --type image/png queries
const stubWlPasteScript = `#!/bin/sh
payload="clipdeck-stub-clipboard-payload"
watch=0
for arg in "$@"; do
	case "$arg" in
	--list-types)
		printf 'text/plain\ntext/plain;charset=utf-8\n'
		exit 0
		;;
	--no-newline)
		printf '%s' "$payload"
		exit 0
		;;
	--watch)
		watch=1
		;;
	esac
done
if [ "$watch" = "1" ]; then
	printf '\n'
	exec sleep 3
fi
exit 0
`

func writeStubWlPaste(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wl-paste")
	if err := os.WriteFile(path, []byte(stubWlPasteScript), 0o755); err != nil {
		t.Fatalf("write stub wl-paste: %v", err)
	}
	return dir
}

// TestWatcherIngestsClipboardChange proves a clipboard change event is
// actually ingested: the watcher must trigger SnapshotCurrent() and store the
// current selection, not just hold a watcher process alive.
//
// Break it catches: a runWatcher that discards the wl-paste stdout stream
// (the historical bug) leaves the store empty forever even though wl-paste
// fires change events on it.
func TestWatcherIngestsClipboardChange(t *testing.T) {
	stubDir := writeStubWlPaste(t)
	t.Setenv("PATH", stubDir+":"+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	if _, err := exec.LookPath("wl-paste"); err != nil {
		t.Fatalf("stub wl-paste not on PATH: %v", err)
	}

	cfg := config.DefaultConfig("")
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	cap := New(cfg, st, nil)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() {
		defer close(started)
		_ = cap.Start(ctx)
	}()
	// Stop the watchers before the store is closed, so an in-flight snapshot
	// can never race teardown. (Registered after st.Close, so it runs first.)
	defer func() {
		cancel()
		<-started
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		items, _, err := st.ListItems(10, 0)
		if err != nil {
			t.Fatalf("list items: %v", err)
		}
		for _, item := range items {
			if item.Content == "clipdeck-stub-clipboard-payload" {
				return // change event reached the store
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("clipboard change was never ingested into the store")
}
