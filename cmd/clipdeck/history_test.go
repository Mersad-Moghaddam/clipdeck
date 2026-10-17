package main

import (
	"bytes"
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
