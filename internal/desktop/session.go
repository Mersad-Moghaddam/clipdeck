// Package desktop provides native-session clipboard and window integration.
package desktop

import "fmt"

// DetectSession prefers the explicitly declared native session over XWayland.
// It rejects an incomplete explicit session rather than using another display.
func DetectSession(getenv func(string) string) (string, error) {
	switch session := getenv("XDG_SESSION_TYPE"); session {
	case "x11":
		if getenv("DISPLAY") != "" {
			return "x11", nil
		}
	case "wayland":
		if getenv("WAYLAND_DISPLAY") != "" {
			return "wayland", nil
		}
	case "":
		if getenv("WAYLAND_DISPLAY") != "" {
			return "wayland", nil
		}
		if getenv("DISPLAY") != "" {
			return "x11", nil
		}
	default:
		return "", fmt.Errorf("unsupported graphical session %q", session)
	}
	return "", fmt.Errorf("no usable graphical display; launch ClipDeck inside your desktop session")
}
