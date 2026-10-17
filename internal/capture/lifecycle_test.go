package capture

import (
	"context"
	"testing"
	"time"
)

func TestRetryWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if retryWait(ctx, time.Hour) {
		t.Fatal("retry allowed after cancellation")
	}
}
