package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWaylandWritePreservesBytesAndMIME(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload")
	args := filepath.Join(dir, "args")
	t.Setenv("PAYLOAD", payload)
	t.Setenv("ARGS", args)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >\"$ARGS\"\ncat >\"$PAYLOAD\"\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	b := NewWaylandBackend()
	if err := b.Write(context.Background(), Selection{MIME: "text/plain", Data: []byte("hello\nworld\n")}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(payload)
	if string(data) != "hello\nworld\n" {
		t.Fatalf("payload %q", data)
	}
	data, _ = os.ReadFile(args)
	if string(data) != "--type\ntext/plain\n" {
		t.Fatalf("args %q", data)
	}
}

func TestWaylandReadRejectsSecretSelection(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = --list-types ]; then printf 'text/plain\\napplication/x-secret-flag\\n'; else printf secret; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "wl-paste"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	selection, err := NewWaylandBackend().Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Data) != 0 {
		t.Fatal("secret selection exposed")
	}
}
