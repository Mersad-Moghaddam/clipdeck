package ipc

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func TestIPCServer(t *testing.T) {
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "clipdeck.sock")

	cfg := &config.Config{
		Storage: config.StorageConfig{
			DataDir: tempDir,
		},
		IPC: config.IPCConfig{
			SocketPath: sockPath,
		},
	}

	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	// Seed an item in store
	item := &store.Item{
		Hash:      "hash123",
		Type:      "text",
		Mime:      "text/plain",
		Content:   "Hello World",
		Preview:   "Hello World",
		SourceApp: "kitty",
	}
	if err := st.SaveItem(item); err != nil {
		t.Fatalf("SaveItem failed: %v", err)
	}

	srv := NewServer(cfg, st, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer srv.Stop()

	// 1. Verify socket permissions
	fi, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket failed: %v", err)
	}
	perm := fi.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected 0600 permissions, got %04o", perm)
	}

	// 2. Connect client
	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// 3. Test Ping
	var pingResp map[string]any
	if err := client.Call("ping", nil, &pingResp); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if pingResp["pong"] != true {
		t.Errorf("expected pong: true, got %v", pingResp)
	}

	// 4. Test Status
	var statusResp map[string]any
	if err := client.Call("status", nil, &statusResp); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if statusResp["version"] == "" || statusResp["paused"] != false {
		t.Errorf("unexpected status response: %v", statusResp)
	}

	// 5. Test List
	var listResp struct {
		Items []store.Item `json:"items"`
		Total int          `json:"total"`
	}
	if err := client.Call("list", map[string]any{"limit": 10, "offset": 0}, &listResp); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 item, got total=%d items=%d", listResp.Total, len(listResp.Items))
	}
	itemID := listResp.Items[0].ID

	// 6. Test Search
	var searchResp struct {
		Items []store.Item `json:"items"`
		Total int          `json:"total"`
	}
	if err := client.Call("search", map[string]any{"query": "Hello", "limit": 10}, &searchResp); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if searchResp.Total != 1 {
		t.Errorf("search expected 1 result, got %d", searchResp.Total)
	}

	// 7. Test Pin
	var pinResp map[string]any
	if err := client.Call("pin", map[string]any{"id": itemID, "pinned": true}, &pinResp); err != nil {
		t.Fatalf("pin failed: %v", err)
	}
	if pinResp["success"] != true {
		t.Errorf("expected success: true, got %v", pinResp)
	}

	// 8. Test Pause
	var pauseResp map[string]any
	if err := client.Call("pause", map[string]any{"paused": true}, &pauseResp); err != nil {
		t.Fatalf("pause failed: %v", err)
	}
	if pauseResp["paused"] != true {
		t.Errorf("expected paused: true, got %v", pauseResp)
	}

	// 9. Test Transform
	var transformResp struct {
		Text string `json:"text"`
	}
	if err := client.Call("transform", map[string]any{"text": "hello world", "action": "uppercase"}, &transformResp); err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if transformResp.Text != "HELLO WORLD" {
		t.Errorf("expected HELLO WORLD, got %s", transformResp.Text)
	}

	// 10. Test Broadcast
	// Connect a raw reader to check incoming broadcast events
	rawConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("raw dial failed: %v", err)
	}
	defer rawConn.Close()

	reader := bufio.NewReader(rawConn)
	time.Sleep(50 * time.Millisecond)

	srv.Broadcast("item_added", map[string]string{"id": "new-123"})

	lineChan := make(chan string, 1)
	go func() {
		line, _ := reader.ReadString('\n')
		lineChan <- line
	}()

	select {
	case line := <-lineChan:
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("unmarshal broadcast event failed: %v", err)
		}
		if event["event"] != "item_added" {
			t.Errorf("expected item_added event, got %v", event["event"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}

	// 11. Test Delete
	var delResp map[string]any
	if err := client.Call("delete", map[string]any{"id": itemID}, &delResp); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if delResp["success"] != true {
		t.Errorf("expected success: true, got %v", delResp)
	}
}

func TestIPC_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "clipdeck.sock")

	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: tempDir},
		IPC:     config.IPCConfig{SocketPath: sockPath},
	}
	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	srv := NewServer(cfg, st, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer srv.Stop()

	client, err := NewClient(sockPath)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	// 1. Unknown method
	var res map[string]any
	err = client.Call("non_existent_method", nil, &res)
	if err == nil {
		t.Fatal("expected error on unknown method, got nil")
	}

	// 2. Get non-existent item
	err = client.Call("get", map[string]any{"id": "does-not-exist"}, &res)
	if err == nil {
		t.Fatal("expected error on missing item, got nil")
	}

	// 3. Raw malformed JSON
	rawConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial raw failed: %v", err)
	}
	defer rawConn.Close()

	_, err = rawConn.Write([]byte("this is not json\n"))
	if err != nil {
		t.Fatalf("write malformed json failed: %v", err)
	}

	reader := bufio.NewReader(rawConn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read malformed response failed: %v", err)
	}
	var rawResp Response
	if err := json.Unmarshal([]byte(line), &rawResp); err != nil {
		t.Fatalf("unmarshal error response failed: %v", err)
	}
	if rawResp.Error == nil || rawResp.Error.Code != -32700 {
		t.Errorf("expected parse error -32700, got: %+v", rawResp.Error)
	}
}

func TestIPC_ConcurrentClients(t *testing.T) {
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "clipdeck.sock")

	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: tempDir},
		IPC:     config.IPCConfig{SocketPath: sockPath},
	}
	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	srv := NewServer(cfg, st, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer srv.Stop()

	const numClients = 10
	errChan := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			c, err := NewClient(sockPath)
			if err != nil {
				errChan <- err
				return
			}
			defer c.Close()

			var statusResp map[string]any
			if err := c.Call("status", nil, &statusResp); err != nil {
				errChan <- err
				return
			}
			errChan <- nil
		}()
	}

	for i := 0; i < numClients; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent client failed: %v", err)
		}
	}
}
