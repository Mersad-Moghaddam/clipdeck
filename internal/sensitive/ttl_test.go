package sensitive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

func TestSensitiveTTLExpiryAndPurge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clipdeck-ttl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	s, err := store.Open(tmpDir)
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	secretPayload := "SuperSensitivePasswordNeverToLeak1234!"
	check := Check(secretPayload, []string{"x-kde-passwordManagerHint"}, "browser", nil)
	if !check.IsSensitive {
		t.Fatalf("Expected check to be sensitive")
	}

	// Save with TTL = 1 second in the past to simulate expiry
	expiredItem := &store.Item{
		Hash:          "test-secret-hash-1",
		Type:          "text",
		Mime:          "text/plain",
		Content:       secretPayload,
		Preview:       check.MaskedText,
		Sensitive:     true,
		MaskedContent: check.MaskedText,
		ExpiresAt:     time.Now().Unix() - 1, // expired
		SizeBytes:     int64(len(secretPayload)),
	}

	if err := s.SaveItem(expiredItem); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	// Run PurgeExpired
	purged, err := s.PurgeExpired()
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged == 0 {
		t.Fatalf("Expected at least 1 purged item, got 0")
	}

	// Verify DB file contains zero plaintext of the secret payload
	dbPath := filepath.Join(tmpDir, "clipdeck.db")
	rawBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile db: %v", err)
	}

	if strings.Contains(string(rawBytes), secretPayload) {
		t.Fatalf("Security failure: DB file contains plaintext of expired sensitive payload after purge!")
	}
}
