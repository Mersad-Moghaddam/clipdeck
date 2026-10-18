package desktop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type x11Backend struct{}

func NewX11Backend() Backend { return &x11Backend{} }
func NativeBackend() (Backend, error) {
	session, err := DetectSession(os.Getenv)
	if err != nil {
		return nil, err
	}
	if session == "x11" {
		return NewX11Backend(), nil
	}
	return NewWaylandBackend(), nil
}
func (*x11Backend) Read(ctx context.Context) (Selection, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	data, err := helperOutput(ctx, "xclip", "-selection", "clipboard", "-out", "-target", "TARGETS")
	if err != nil {
		return Selection{}, err
	}
	available := map[string]bool{}
	for _, s := range strings.Fields(string(data)) {
		available[s] = true
	}
	if available["application/x-secret-flag"] || available["x-kde-passwordManagerHint"] {
		return Selection{}, nil
	}
	for _, target := range []string{"image/png", "image/jpeg", "text/uri-list", "UTF8_STRING", "text/plain;charset=utf-8", "text/plain", "STRING"} {
		if !available[target] {
			continue
		}
		data, err := helperOutput(ctx, "xclip", "-selection", "clipboard", "-out", "-target", target)
		if err != nil {
			continue
		}
		mime := target
		if target == "UTF8_STRING" || target == "STRING" {
			mime = "text/plain;charset=utf-8"
		}
		return Selection{MIME: mime, Data: data}, nil
	}
	return Selection{}, nil
}
func (*x11Backend) Write(ctx context.Context, s Selection) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	target := s.MIME
	if strings.HasPrefix(target, "text/plain") {
		target = "UTF8_STRING"
	}
	// xclip's default silent mode forks a selection owner after ingesting stdin.
	// Do not attach stdout/stderr pipes: the owner would keep them open.
	cmd := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-in", "-target", target)
	cmd.Stdin = bytes.NewReader(s.Data)
	return cmd.Run()
}
func (b *x11Backend) Watch(ctx context.Context, changed func()) error {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	var last Selection
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s, err := b.Read(ctx)
			if err != nil {
				continue
			}
			if s.MIME != last.MIME || !bytes.Equal(s.Data, last.Data) {
				last = s
				changed()
			}
		}
	}
}
func (*x11Backend) ActiveWindow(ctx context.Context) (Window, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	data, err := helperOutput(ctx, "xdotool", "getactivewindow")
	if err != nil {
		return Window{}, err
	}
	id := strings.TrimSpace(string(data))
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		return Window{}, fmt.Errorf("invalid X11 window ID")
	}
	title, _ := helperOutput(ctx, "xdotool", "getwindowname", id)
	class, _ := helperOutput(ctx, "xprop", "-id", id, "WM_CLASS")
	value := strings.TrimSpace(string(class))
	if parts := strings.Split(value, "\""); len(parts) >= 4 {
		value = parts[len(parts)-2]
	} else {
		value = ""
	}
	return Window{ID: id, Class: value, Title: strings.TrimSpace(string(title))}, nil
}
func (*x11Backend) Paste(ctx context.Context, w Window) error {
	if id, err := strconv.ParseUint(w.ID, 10, 64); err != nil || id == 0 {
		return fmt.Errorf("paste requires a valid original X11 window")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	class, err := helperOutput(ctx, "xprop", "-id", w.ID, "WM_CLASS")
	if err != nil {
		return fmt.Errorf("verify paste target identity: %w", err)
	}
	for _, field := range strings.Split(string(class), "\"") {
		if strings.EqualFold(strings.TrimSpace(field), "clipdeck-ui") {
			return fmt.Errorf("paste target is the ClipDeck popup; copy and paste manually")
		}
	}
	if err := exec.CommandContext(ctx, "xdotool", "windowactivate", "--sync", w.ID).Run(); err != nil {
		return fmt.Errorf("restore target: %w", err)
	}
	active, err := helperOutput(ctx, "xdotool", "getactivewindow")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(active)) != w.ID {
		return fmt.Errorf("paste target lost focus; copied, paste manually")
	}
	return exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v").Run()
}
