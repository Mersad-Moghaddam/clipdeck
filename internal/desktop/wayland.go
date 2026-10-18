package desktop

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/hypr"
)

type waylandBackend struct{}

func NewWaylandBackend() Backend { return &waylandBackend{} }
func (*waylandBackend) Read(ctx context.Context) (Selection, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := helperOutput(ctx, "wl-paste", "--list-types")
	if err != nil {
		return Selection{}, err
	}
	types := strings.Fields(string(data))
	available := map[string]bool{}
	for _, mime := range types {
		available[mime] = true
	}
	if available["x-kde-passwordManagerHint"] || available["application/x-secret-flag"] {
		return Selection{}, nil
	}
	for _, mime := range []string{"image/png", "image/jpeg", "image/webp", "text/uri-list", "text/plain;charset=utf-8", "text/plain", "UTF8_STRING"} {
		if available[mime] {
			data, err = helperOutput(ctx, "wl-paste", "--no-newline", "--type", mime)
			if err == nil {
				return Selection{MIME: mime, Data: data}, nil
			}
		}
	}
	return Selection{}, nil
}
func (*waylandBackend) Write(ctx context.Context, s Selection) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wl-copy", "--type", s.MIME)
	cmd.Stdin = bytes.NewReader(s.Data)
	return cmd.Run()
}
func (b *waylandBackend) Watch(ctx context.Context, changed func()) error {
	// Polling is deliberately encapsulated; the existing compositor watcher
	// remains in use until capture integration is covered by regression tests.
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	var last Selection
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s, err := b.Read(ctx)
			if err != nil {
				continue
			}
			if s.MIME != last.MIME || !bytes.Equal(s.Data, last.Data) {
				last = s
				changed()
			}
		}
	}
}
func (*waylandBackend) ActiveWindow(ctx context.Context) (Window, error) {
	if err := ctx.Err(); err != nil {
		return Window{}, err
	}
	w, err := hypr.GetActiveWindow()
	return Window{Class: w.Class, Title: w.Title}, err
}
func (*waylandBackend) Paste(ctx context.Context, _ Window) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return hypr.PasteDirectly()
}
