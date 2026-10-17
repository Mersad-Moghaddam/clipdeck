package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func TestPasteReportsRequestedNotDelivered(t *testing.T) {
	srv, st, _, _ := newHardeningServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte("#!/bin/sh\n/bin/cat >/dev/null\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	item := &store.Item{Hash: "paste", Type: "text", Mime: "text/plain", Content: "synthetic", Preview: "synthetic", SizeBytes: 9}
	if err := st.SaveItem(item); err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(PasteParams{ID: item.ID})
	res, rpcErr := srv.handlePaste(nil, params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := res.(map[string]any)
	if got["copied"] != true || got["paste_requested"] != true {
		t.Fatalf("misleading response: %+v", got)
	}
}

func TestQueryMasksSecrets(t *testing.T) {
	_, st, path, _ := newHardeningServer(t)
	item := &store.Item{Hash: "secret", Type: "text", Mime: "text/plain", Content: "secret-value", Preview: "secret-value", RichPreview: "secret-value", Sensitive: true, MaskedContent: "[hidden]", SizeBytes: 12}
	if err := st.SaveItem(item); err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var page store.QueryPage
	if err := c.Call("query", store.QueryOptions{Limit: 1}, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatal(page)
	}
	got := page.Items[0]
	if got.Content != "[hidden]" || got.Preview != "[hidden]" || got.RichPreview != "" {
		t.Fatalf("secret leaked: %+v", got)
	}
	if err := c.Call("query", map[string]any{"limit": 201}, &page); err == nil {
		t.Fatal("accepted excessive limit")
	}
}

func TestPasteFailureTargetedToCaller(t *testing.T) {
	_, st, sockPath, _ := newHardeningServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte("#!/bin/sh\n/bin/cat >/dev/null\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	item := &store.Item{Hash: "paste1", Type: "text", Mime: "text/plain", Content: "synthetic", Preview: "synthetic", SizeBytes: 9}
	if err := st.SaveItem(item); err != nil {
		t.Fatal(err)
	}

	c1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()

	c2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	// Client 1 calls paste
	req := fmt.Sprintf(`{"id":"p1","method":"paste","params":{"id":"%s"}}`+"\n", item.ID)
	if _, err := c1.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}

	// Read response on client 1
	r1 := bufio.NewReader(c1)
	respLine, err := r1.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(respLine, `"success":true`) {
		t.Fatalf("expected success response, got: %s", respLine)
	}

	// Client 2 must NOT receive paste_failed broadcast
	_ = c2.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
	r2 := bufio.NewReader(c2)
	line, err := r2.ReadString('\n')
	if err == nil {
		t.Fatalf("client 2 unexpectedly received event: %s", line)
	}
}
