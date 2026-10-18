package desktop

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestX11RealClipboardRoundTrip(t *testing.T) {
	if os.Getenv("CLIPDECK_TEST_X11") != "1" {
		t.Skip("run scripts/test-x11.sh for real X11 integration")
	}
	b := NewX11Backend()
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Selection{{MIME: "text/plain;charset=utf-8", Data: []byte("hello X11\nλ\n")}, {MIME: "image/png", Data: pngData.Bytes()}} {
		if err := b.Write(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			got, err := b.Read(context.Background())
			if err == nil && got.MIME == s.MIME && bytes.Equal(got.Data, s.Data) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("roundtrip mime=%s got=%+v err=%v", s.MIME, got, err)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Watch(ctx, func() { t.Error("callback after cancellation") }); err != context.Canceled {
		t.Fatalf("watch cancellation=%v", err)
	}
}

func TestX11RealTargetPaste(t *testing.T) {
	target := os.Getenv("CLIPDECK_TEST_PASTE_TARGET")
	if target == "" {
		t.Skip("run scripts/test-i3.sh for target paste integration")
	}
	b := NewX11Backend()
	if err := b.Write(context.Background(), Selection{MIME: "text/plain", Data: []byte("ClipDeck target paste λ")}); err != nil {
		t.Fatal(err)
	}
	if err := b.Paste(context.Background(), Window{ID: target}); err != nil {
		t.Fatal(err)
	}
}
