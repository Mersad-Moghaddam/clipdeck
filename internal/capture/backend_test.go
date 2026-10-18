package capture

import (
	"context"
	"errors"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/desktop"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

type selectionBackend struct {
	desktop.Backend
	selection desktop.Selection
	err       error
}

func (b selectionBackend) Read(context.Context) (desktop.Selection, error) { return b.selection, b.err }

func (b selectionBackend) ActiveWindow(context.Context) (desktop.Window, error) {
	return desktop.Window{}, nil
}

func TestSnapshotUsesDesktopBackend(t *testing.T) {
	cfg := config.DefaultConfig("")
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := New(cfg, st, nil)
	c.backend = selectionBackend{selection: desktop.Selection{MIME: "text/plain", Data: []byte("native backend payload")}}
	item, err := c.SnapshotCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.Content != "native backend payload" {
		t.Fatalf("item=%+v", item)
	}
	c.backend = selectionBackend{err: errors.New("clipboard owner unavailable")}
	if _, err = c.SnapshotCurrent(); err == nil {
		t.Fatal("backend failure ignored")
	}
}

func TestX11CaptureSelectsNativeBackend(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("DISPLAY", ":99")
	c := New(config.DefaultConfig(""), nil, nil)
	if c.backend == nil {
		t.Fatal("X11 capture backend not selected")
	}
}

func TestNativeBackendPreservesFileSelection(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := New(config.DefaultConfig(""), st, nil)
	c.backend = selectionBackend{selection: desktop.Selection{MIME: "text/uri-list", Data: []byte("# file selection\r\nfile:///home/user/hello%20world.txt\r\n")}}
	item, err := c.SnapshotCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.Type != "file" || item.Content != "/home/user/hello world.txt" {
		t.Fatalf("item=%+v", item)
	}
	c.SetPaused(true)
	if item, err := c.SnapshotCurrent(); err != nil || item != nil {
		t.Fatalf("paused capture item=%+v error=%v", item, err)
	}
}

func TestNativeBackendRetainsSourceApp(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := New(config.DefaultConfig(""), st, nil)
	c.backend = attributedBackend{selectionBackend: selectionBackend{selection: desktop.Selection{MIME: "text/plain", Data: []byte("attributed selection")}}}
	item, err := c.SnapshotCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.SourceApp != "editor" {
		t.Fatalf("item=%+v", item)
	}
}

type attributedBackend struct{ selectionBackend }

func (attributedBackend) ActiveWindow(context.Context) (desktop.Window, error) {
	return desktop.Window{Class: "editor"}, nil
}
