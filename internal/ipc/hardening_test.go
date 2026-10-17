package ipc

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func newHardeningServer(t *testing.T) (*Server, *store.Store, string, *config.Config) {
	t.Helper()
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "clipdeck.sock")
	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: tempDir},
		IPC:     config.IPCConfig{SocketPath: sockPath},
	}
	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv := NewServer(cfg, st, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("server start: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv, st, sockPath, cfg
}

// List/search stream up to hundreds of items; their per-item content and
// rich previews must be trimmed so one line can never exceed the 10 MiB IPC
// scan limit. Full content stays available through the get RPC.
func TestListResponseSlimsOversizedContent(t *testing.T) {
	_, st, sockPath, _ := newHardeningServer(t)

	big := strings.Repeat("a", 100*1024)
	item := &store.Item{Hash: "big", Type: "text", Mime: "text/plain", Content: big, Preview: "big", SizeBytes: int64(len(big))}
	if err := st.SaveItem(item); err != nil {
		t.Fatalf("seed: %v", err)
	}

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	var res struct {
		Items []store.Item `json:"items"`
		Total int          `json:"total"`
	}
	if err := client.Call("list", map[string]any{"limit": 10, "offset": 0}, &res); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if got := len(res.Items[0].Content); got > maxListStreamContent+4 {
		t.Fatalf("list content must be trimmed (got %d bytes)", got)
	}
	if !strings.HasSuffix(res.Items[0].Content, "\u2026") {
		t.Fatalf("trimmed content should be marked with an ellipsis, got len %d", len(res.Items[0].Content))
	}

	// get keeps delivering the full payload for id-based actions.
	var getRes struct {
		Item store.Item `json:"item"`
	}
	if err := client.Call("get", map[string]any{"id": item.ID}, &getRes); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(getRes.Item.Content) != len(big) {
		t.Fatalf("get must return full content (got %d)", len(getRes.Item.Content))
	}
}

func TestGetUnknownItemReturnsNotFoundCode(t *testing.T) {
	_, _, sockPath, _ := newHardeningServer(t)

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	var sink map[string]any
	callErr := client.Call("get", map[string]any{"id": "does-not-exist"}, &sink)
	if callErr == nil {
		t.Fatal("get with unknown id must error")
	}
	if !strings.Contains(callErr.Error(), "-32004") && !strings.Contains(callErr.Error(), "not found") {
		t.Fatalf("expected item-not-found error (-32004), got: %v", callErr)
	}
}

// A client that stops reading must not stall broadcasts for everyone else:
// the write gets a deadline and the wedged connection is dropped.
func TestBroadcastDoesNotStallOnWedgedClient(t *testing.T) {
	srv, _, sockPath, _ := newHardeningServer(t)

	// Active client that keeps reading responses.
	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	// Actually drain broadcast bytes continuously so the client's receive
	// buffer never fills (otherwise even the healthy client hits the write
	// deadline — correct server behavior).
	drainStop := make(chan struct{})
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		drainBuf := make([]byte, 64*1024)
		for {
			select {
			case <-drainStop:
				return
			default:
			}
			_ = client.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			if _, rerr := client.conn.Read(drainBuf); rerr != nil {
				continue
			}
		}
	}()

	// Wedged client: accepted, registered, never reads.
	wedged, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial wedged client: %v", err)
	}
	defer wedged.Close()

	// Wait until both connections are registered.
	deadline := time.Now().Add(3 * time.Second)
	for {
		srv.clientsMu.Lock()
		n := len(srv.clients)
		srv.clientsMu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("connections not registered (n=%d)", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	big := make([]byte, 2*1024*1024) // far larger than the socket buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Broadcast("item_added", string(big))
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("Broadcast blocked beyond the write deadline on a wedged client")
	}

	// After eviction the wedged connection must read through any residual
	// buffered bytes to a hard read error (EOF / dead connection).
	wedged.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	dropped := false
	for {
		if _, rerr := wedged.Read(buf); rerr != nil {
			dropped = true
			break
		}
	}
	if !dropped {
		t.Fatal("wedged client should have been dropped by the server")
	}

	// The healthy client still works.
	close(drainStop)
	select {
	case <-drainDone: // drain fully exited: no more competing reads
	case <-time.After(1200 * time.Millisecond):
		t.Fatal("drain goroutine did not stop")
	}
	var pong map[string]any
	if cerr := client.Call("ping", nil, &pong); cerr != nil {
		t.Fatalf("post-eviction ping failed: %v", cerr)
	}
	if pong["pong"] != true {
		t.Fatalf("unexpected ping response: %v", pong)
	}
}
