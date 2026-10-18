package ipc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPasteParamsPreserveConfirmedTarget(t *testing.T) {
	var p PasteParams
	if err := json.Unmarshal([]byte(`{"id":"a","confirmed":true,"target_window":"42"}`), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Confirmed || p.TargetWindow != "42" {
		t.Fatalf("paste params=%+v", p)
	}
}

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
