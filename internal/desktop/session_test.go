package desktop

import "testing"

func TestDetectSession(t *testing.T) {
	for _, tc := range []struct {
		name, session, wayland, display, want string
		wantErr                               bool
	}{
		{"native X11 despite Wayland variable", "x11", "wayland-1", ":0", "x11", false},
		{"native Wayland with XWayland", "wayland", "wayland-1", ":0", "wayland", false},
		{"infer Wayland", "", "wayland-1", ":0", "wayland", false},
		{"infer X11", "", "", ":0", "x11", false},
		{"headless", "", "", "", "", true},
		{"broken explicit X11", "x11", "wayland-1", "", "", true},
		{"broken explicit Wayland", "wayland", "", ":0", "", true},
		{"unsupported session", "tty", "", ":0", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"XDG_SESSION_TYPE": tc.session, "WAYLAND_DISPLAY": tc.wayland, "DISPLAY": tc.display}
			got, err := DetectSession(func(key string) string { return env[key] })
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
