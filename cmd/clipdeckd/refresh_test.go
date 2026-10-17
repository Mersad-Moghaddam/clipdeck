package main

import (
	"context"
	"strings"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

// The startup markup refresh regenerates code/json previews that predate the
// class-based markup. It must NOT resurrect raw markup for sensitive rows:
// those ship without rich previews in the current capture pipeline.
func TestRefreshLegacyMarkupSkipsSensitive(t *testing.T) {
	dataDir := t.TempDir()
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	plainCode := "def hello():\n    return 42"
	secretCode := "-----BEGIN RSA PRIVATE KEY-----\nQWJDQWJD\n-----END RSA PRIVATE KEY-----"

	plain := &store.Item{
		Hash:        "legacy-plain",
		Type:        "code",
		Mime:        "text/plain",
		Content:     plainCode,
		Preview:     "def hello()…",
		RichPreview: "<span class=\"old-inline-style\">def hello():</span>",
		SourceApp:   "kitty",
		SizeBytes:   int64(len(plainCode)),
	}
	secret := &store.Item{
		Hash:        "legacy-secret",
		Type:        "code",
		Mime:        "text/plain",
		Content:     secretCode,
		Preview:     "masked",
		RichPreview: "<span class=\"old-inline-style\">-----BEGIN RSA PRIVATE KEY-----</span>",
		SourceApp:   "kitty",
		Sensitive:   true,
		SizeBytes:   int64(len(secretCode)),
	}
	if err := st.SaveItem(plain); err != nil {
		t.Fatalf("seed plain: %v", err)
	}
	if err := st.SaveItem(secret); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	// Seed rows as they look in an old database: unmasked code markup.
	st.RefreshRichPreview(plain.ID, "<span class=\"old-inline-style\">def hello():</span>")
	st.RefreshRichPreview(secret.ID, "<span class=\"old-inline-style\">-----BEGIN RSA PRIVATE KEY-----</span>")

	refreshLegacyMarkup(context.Background(), st)

	gotPlain, err := st.GetItem(plain.ID)
	if err != nil {
		t.Fatalf("get plain: %v", err)
	}
	gotSecret, err := st.GetItem(secret.ID)
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}

	if gotPlain.RichPreview == "<span class=\"old-inline-style\">def hello():</span>" {
		t.Fatal("legacy plain markup should have been refreshed")
	}
	if !strings.Contains(gotPlain.RichPreview, "t-") {
		t.Fatalf("plain legacy item should receive class-based markup, got: %.80s", gotPlain.RichPreview)
	}
	if gotSecret.RichPreview != "" {
		t.Fatalf("sensitive legacy item must not get any rich markup, got %.80s", gotSecret.RichPreview)
	}
}
