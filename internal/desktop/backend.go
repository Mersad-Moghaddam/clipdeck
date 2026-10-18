package desktop

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const MaxSelectionBytes = 32 << 20

type Selection struct {
	MIME string
	Data []byte
}
type Window struct{ ID, Class, Title string }
type Backend interface {
	Read(context.Context) (Selection, error)
	Write(context.Context, Selection) error
	Watch(context.Context, func()) error
	ActiveWindow(context.Context) (Window, error)
	Paste(context.Context, Window) error
}

// helperOutput bounds retained output even when a clipboard owner is hostile.
func helperOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Descendants may inherit stdout even after the immediate helper is killed.
	// Bound waiting for the copy goroutine as well as the process itself.
	cmd.WaitDelay = 100 * time.Millisecond
	var out limitedBuffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > MaxSelectionBytes-b.Len() {
		return 0, fmt.Errorf("clipboard exceeds %d bytes", MaxSelectionBytes)
	}
	return b.Buffer.Write(p)
}

var _ io.Writer = (*limitedBuffer)(nil)
