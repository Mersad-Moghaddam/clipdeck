package desktop

import (
	"context"
	"testing"
	"time"
)

func TestHelperCancellationBoundsInheritedOutputPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := helperOutput(ctx, "sh", "-c", "sleep 2 & wait")
	if err == nil {
		t.Fatal("expected cancelled helper error")
	}
	if time.Since(start) > time.Second {
		t.Fatal("descendant retained helper pipe after cancellation")
	}
}
