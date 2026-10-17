//go:build integration

package test

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func TestIntegration_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	sockDir := filepath.Join(tempDir, "run")
	dataDir := filepath.Join(tempDir, "data")
	_ = os.MkdirAll(sockDir, 0700)
	_ = os.MkdirAll(dataDir, 0700)
	sockPath := filepath.Join(sockDir, "clipdeck.sock")

	// Build clipdeckd and clipdeck binaries for testing
	clipdeckdBin := filepath.Join(tempDir, "clipdeckd")
	clipdeckBin := filepath.Join(tempDir, "clipdeck")

	buildCmd := exec.Command("go", "build", "-o", clipdeckdBin, "../cmd/clipdeckd")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build clipdeckd: %v\nOutput: %s", err, string(out))
	}

	buildCliCmd := exec.Command("go", "build", "-o", clipdeckBin, "../cmd/clipdeck")
	if out, err := buildCliCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build clipdeck: %v\nOutput: %s", err, string(out))
	}

	// Prepare config file
	configContent := strings.ReplaceAll(`
[storage]
data_dir = "DATADIR"
max_items = 1000
max_age_days = 7
max_size_bytes = 10485760

[capture]
max_text_bytes = 1048576
max_image_bytes = 5242880
excluded_apps = ["1Password"]
debounce_ms = 50

[privacy]
auto_mask_sensitive = true
sensitive_ttl_sec = 2
check_luhn = true
check_api_keys = true
check_private_keys = true
fetch_link_titles = false

[ipc]
socket_path = "SOCKPATH"
`, "DATADIR", dataDir)
	configContent = strings.ReplaceAll(configContent, "SOCKPATH", sockPath)

	configFile := filepath.Join(tempDir, "config.toml")
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Start clipdeckd with --no-capture and test config
	daemonCmd := exec.Command(clipdeckdBin, "-config", configFile, "-no-capture")
	daemonCmd.Stdout = os.Stdout
	daemonCmd.Stderr = os.Stderr
	if err := daemonCmd.Start(); err != nil {
		t.Fatalf("failed to start daemon: %v", err)
	}
	defer func() {
		_ = daemonCmd.Process.Kill()
		_ = daemonCmd.Wait()
	}()

	// Wait for socket to become ready
	ready := false
	for i := 0; i < 50; i++ {
		time.Sleep(50 * time.Millisecond)
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("daemon failed to create socket at %s", sockPath)
	}

	// 1. Connect IPC client
	client, err := ipc.NewClient(sockPath)
	if err != nil {
		t.Fatalf("ipc.NewClient failed: %v", err)
	}
	defer client.Close()

	// 2. Query status
	var status struct {
		Version     string `json:"version"`
		Paused      bool   `json:"paused"`
		TotalItems  int    `json:"total_items"`
		PinnedItems int    `json:"pinned_items"`
	}
	if err := client.Call("status", nil, &status); err != nil {
		t.Fatalf("call status failed: %v", err)
	}
	if status.Version == "" || status.TotalItems != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}

	// 3. Directly seed items into the store to test IPC retrieval
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	item1 := &store.Item{
		Hash:      "hash-go-code",
		Type:      "code",
		Mime:      "text/plain",
		Content:   "package main\n\nfunc main() {}",
		Preview:   "package main",
		SourceApp: "kitty",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := st.SaveItem(item1); err != nil {
		t.Fatalf("save item1 failed: %v", err)
	}

	item2 := &store.Item{
		Hash:      "hash-persian-text",
		Type:      "text",
		Mime:      "text/plain",
		Content:   "متن تستی فارسی برای جستجو",
		Preview:   "متن تستی فارسی",
		SourceApp: "telegram-desktop",
		Pinned:    true,
		PinnedAt:  time.Now().Unix(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := st.SaveItem(item2); err != nil {
		t.Fatalf("save item2 failed: %v", err)
	}

	// 4. Query list via IPC
	var listResp struct {
		Items []store.Item `json:"items"`
		Total int          `json:"total"`
	}
	if err := client.Call("list", map[string]any{"limit": 10}, &listResp); err != nil {
		t.Fatalf("call list failed: %v", err)
	}
	if listResp.Total != 2 {
		t.Fatalf("expected 2 items, got %d", listResp.Total)
	}

	// 5. Search Persian text via FTS5 trigram
	var searchResp struct {
		Items []store.Item `json:"items"`
		Total int          `json:"total"`
	}
	if err := client.Call("search", map[string]any{"query": "فارسی"}, &searchResp); err != nil {
		t.Fatalf("call search failed: %v", err)
	}
	if searchResp.Total != 1 || searchResp.Items[0].SourceApp != "telegram-desktop" {
		t.Fatalf("search expected 1 Persian item, got %d", searchResp.Total)
	}

	// 6. Test Export via CLI tool
	exportFile := filepath.Join(tempDir, "export.json")
	exportCmd := exec.Command(clipdeckBin, "--config", configFile, "export", "--output", exportFile)
	out, err := exportCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI export failed: %v, out: %s", err, string(out))
	}
	exportData, err := os.ReadFile(exportFile)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	var exportedItems []store.Item
	if err := json.Unmarshal(exportData, &exportedItems); err != nil {
		t.Fatalf("failed to parse exported JSON: %v", err)
	}
	if len(exportedItems) != 2 {
		t.Fatalf("expected 2 exported items, got %d", len(exportedItems))
	}

	// 7. Test clear via IPC
	var clearResp map[string]any
	if err := client.Call("clear", map[string]any{"include_pinned": false}, &clearResp); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if clearResp["deleted"].(float64) != 1 {
		t.Fatalf("expected 1 unpinned item deleted, got %v", clearResp["deleted"])
	}

	// Verify pinned item remains
	_ = client.Call("list", map[string]any{"limit": 10}, &listResp)
	if listResp.Total != 1 || !listResp.Items[0].Pinned {
		t.Fatalf("expected pinned item to remain, got total %d", listResp.Total)
	}
}
