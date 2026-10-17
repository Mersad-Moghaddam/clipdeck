package hypr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ActiveWindowInfo struct {
	Class        string `json:"class"`
	Title        string `json:"title"`
	InitialClass string `json:"initialClass"`
	PID          int    `json:"pid"`
}

func GetActiveWindow() (ActiveWindowInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	return getActiveWindow(ctx)
}

func getActiveWindow(ctx context.Context) (ActiveWindowInfo, error) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return ActiveWindowInfo{}, nil
	}
	cmd := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j")
	out, err := cmd.Output()
	if err != nil {
		return ActiveWindowInfo{}, err
	}

	var info ActiveWindowInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return ActiveWindowInfo{}, err
	}

	if info.Class == "" && info.InitialClass != "" {
		info.Class = info.InitialClass
	}

	return info, nil
}

func PasteDirectly() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return pasteDirectly(ctx)
}

func pasteDirectly(ctx context.Context) error {
	// Brief delay to allow overlay to yield focus to the active target window
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(80 * time.Millisecond):
	}

	// First attempt: wtype (standard Wayland virtual keyboard)
	cmd := exec.CommandContext(ctx, "wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl")
	if err := cmd.Run(); err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return fmt.Errorf("copied; paste injection unavailable in this session")
	}

	// Fallback: hyprctl dispatch sendshortcut
	cmd = exec.CommandContext(ctx, "hyprctl", "dispatch", "sendshortcut", "CTRL,V,activewindow")
	return cmd.Run()
}

func SanitizeAppClass(raw string) string {
	raw = strings.TrimSpace(raw)
	// Some apps report reverse domains e.g. "org.mozilla.firefox" -> "firefox"
	parts := strings.Split(raw, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return raw
}
