package hypr

import (
	"context"
	"testing"
	"time"
)

func TestActiveWindowWrongSession(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	info, err := getActiveWindow(ctx)
	if err != nil || info.Class != "" {
		t.Fatalf("expected unavailable attribution, got %+v %v", info, err)
	}
}

func TestPasteCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pasteDirectly(ctx); err == nil {
		t.Fatal("accepted canceled paste")
	}
}
