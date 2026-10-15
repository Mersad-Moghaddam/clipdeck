package hypr

import (
	"encoding/json"
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
	cmd := exec.Command("hyprctl", "activewindow", "-j")
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
	// Brief delay to allow overlay to yield focus to the active target window
	time.Sleep(80 * time.Millisecond)

	// First attempt: wtype (standard Wayland virtual keyboard)
	cmd := exec.Command("wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: hyprctl dispatch sendshortcut
	cmd = exec.Command("hyprctl", "dispatch", "sendshortcut", "CTRL,V,activewindow")
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
