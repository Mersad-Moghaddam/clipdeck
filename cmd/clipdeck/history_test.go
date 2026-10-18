package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func TestHistoryCommandValidation(t *testing.T) {
	for _, args := range [][]string{{"pick", "--json"}, {"list", "--paste"}, {"copy", "--tool", "fzf", "id"}, {"unknown"}, {"list", "--limit", "0"}} {
		var out, diag bytes.Buffer
		if code := runHistoryCommand(config.DefaultConfig("dev"), args, &out, &diag); code != 2 {
			t.Errorf("%v: got %d, %s", args, code, diag.String())
		}
	}
}

func TestCopyCommandFlagAcceptance(t *testing.T) {
	cfg := config.DefaultConfig("dev")
	cfg.IPC.SocketPath = filepath.Join(t.TempDir(), "nonexistent.sock")
	for _, args := range [][]string{
		{"copy", "item-123", "--plain"},
		{"copy", "--plain", "item-123"},
		{"copy", "item-123"},
	} {
		var out, diag bytes.Buffer
		code := runHistoryCommand(cfg, args, &out, &diag)
		// Should reach the daemon connect step (code 1), NOT flag validation failure (code 2)
		if code != 1 {
			t.Errorf("%v: expected code 1 (daemon offline), got %d (%s)", args, code, diag.String())
		}
	}
}

func TestPickerSanitizesControls(t *testing.T) {
	got := pickerLabel("hello\nworld\t\x1b[31m\u202esecret")
	for _, bad := range []string{"\n", "\t", "\x1b", "\u202e"} {
		if strings.Contains(got, bad) {
			t.Fatalf("unsafe label %q", got)
		}
	}
	if len([]rune(pickerLabel(strings.Repeat("a", 500)))) > 160 {
		t.Fatal("unbounded label")
	}
}

func TestPickerExactRowMapping(t *testing.T) {
	rows := []string{"1 [text] same", "2 [text] same"}
	if i, err := selectedRow(rows, "2 [text] same\n"); err != nil || i != 1 {
		t.Fatalf("%d %v", i, err)
	}
	if _, err := selectedRow(rows, "2 forged"); err == nil {
		t.Fatal("accepted forged output")
	}
}

func TestPickerMissingDaemon(t *testing.T) {
	cfg := config.DefaultConfig("dev")
	cfg.IPC.SocketPath = filepath.Join(t.TempDir(), "nonexistent.sock")
	var out, diag bytes.Buffer
	code := runHistoryCommand(cfg, []string{"list"}, &out, &diag)
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(diag.String(), "Daemon unavailable") {
		t.Fatalf("unexpected message: %s", diag.String())
	}
}

func TestListMaskedJSON(t *testing.T) {
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "clipdeck.sock")
	cfg := &config.Config{
		Storage: config.StorageConfig{DataDir: tempDir},
		IPC:     config.IPCConfig{SocketPath: sockPath},
	}
	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	secretItem := &store.Item{
		Hash:          "sec",
		Type:          "text",
		Mime:          "text/plain",
		Content:       "my-super-secret-password-1234",
		Preview:       "my-super-secret-password-1234",
		Sensitive:     true,
		MaskedContent: "[hidden secret]",
		SizeBytes:     29,
	}
	if err := st.SaveItem(secretItem); err != nil {
		t.Fatal(err)
	}

	srv := ipc.NewServer(cfg, st, nil)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	var out, diag bytes.Buffer
	code := runHistoryCommand(cfg, []string{"list", "--json"}, &out, &diag)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, diag.String())
	}

	jsonStr := out.String()
	if strings.Contains(jsonStr, "my-super-secret-password-1234") {
		t.Fatalf("sensitive raw content leaked in json output: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "[hidden secret]") {
		t.Fatalf("expected masked content in json output: %s", jsonStr)
	}
}

func TestImportSanitizesHostileJSON(t *testing.T) {
	tempDir := t.TempDir()
	st, err := store.Open(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	victimFile := filepath.Join(t.TempDir(), "important_user_data.txt")
	if err := os.WriteFile(victimFile, []byte("sensitive-payload"), 0600); err != nil {
		t.Fatal(err)
	}

	validBlob, err := st.SaveBlob([]byte("valid-blob-data"), "png")
	if err != nil {
		t.Fatal(err)
	}

	blobsDir := filepath.Join(tempDir, "blobs")
	rel, err := filepath.Rel(blobsDir, victimFile)
	if err != nil {
		t.Fatal(err)
	}
	traversalPath := blobsDir + "/" + rel

	jsonContent := fmt.Sprintf(`[
		{
			"id": "hostile-1",
			"hash": "h1",
			"type": "image",
			"mime": "image/png",
			"content": "%s",
			"blob_path": "%s",
			"thumbnail_path": "%s"
		},
		{
			"id": "valid-1",
			"hash": "v1",
			"type": "image",
			"mime": "image/png",
			"content": "%s",
			"blob_path": "%s"
		}
	]`, traversalPath, traversalPath, traversalPath, validBlob, validBlob)

	var items []store.Item
	if err := json.Unmarshal([]byte(jsonContent), &items); err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		st.SanitizeImportedItem(&item)
		if err := st.SaveItem(&item); err != nil {
			t.Fatal(err)
		}
	}

	// Verify hostile item had its paths stripped
	hItem, err := st.GetItem("hostile-1")
	if err != nil {
		t.Fatal(err)
	}
	if hItem.BlobPath != "" || hItem.ThumbnailPath != "" {
		t.Fatalf("hostile paths were stored: blob=%q thumb=%q", hItem.BlobPath, hItem.ThumbnailPath)
	}

	// Verify valid item kept its path
	vItem, err := st.GetItem("valid-1")
	if err != nil {
		t.Fatal(err)
	}
	if vItem.BlobPath != validBlob {
		t.Fatalf("valid blob path was dropped: %q", vItem.BlobPath)
	}

	// Now delete hostile item, then clear all
	if err := st.DeleteItem("hostile-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Clear(true); err != nil {
		t.Fatal(err)
	}

	// Victim file must remain intact!
	if _, err := os.Stat(victimFile); err != nil {
		t.Fatalf("victim file was unlinked: %v", err)
	}

	// Valid blob should have been reaped
	if _, err := os.Stat(validBlob); !os.IsNotExist(err) {
		t.Fatalf("valid blob was not reaped after Clear: err=%v", err)
	}
}
