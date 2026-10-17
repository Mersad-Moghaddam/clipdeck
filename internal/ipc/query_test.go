package ipc

import (
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
	"testing"
)

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
