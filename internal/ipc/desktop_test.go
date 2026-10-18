package ipc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyUsesX11Clipboard(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("DISPLAY", ":99")
	t.Setenv("PAYLOAD", payload)
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte("#!/bin/sh\ncat >\"$PAYLOAD\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := copyToClipboard("X11 copied text", "text/plain"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "X11 copied text" {
		t.Fatalf("payload %q", got)
	}
}
