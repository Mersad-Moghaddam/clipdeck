package capture

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PauseStateFile returns the daemon's pause-state file path:
// $XDG_STATE_HOME/clipdeck/paused (dev profile: clipdeck-dev/paused). The
// production isolation keeps the dev profile from toggling the real daemon.
func PauseStateFile(profile string) string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, _ := os.UserHomeDir()
		stateHome = filepath.Join(home, ".local", "state")
	}

	subDir := "clipdeck"
	if profile == "dev" {
		subDir = "clipdeck-dev"
	}
	return filepath.Join(stateHome, subDir, "paused")
}

// SetPauseStateFile enables persistence: every pause toggle rewrites the
// state file so the flag survives daemon restarts.
func (c *Capturer) SetPauseStateFile(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pauseStateFile = path
}

// persistPauseStateLocked rewrites the pause-state file. Callers hold c.mu.
func (c *Capturer) persistPauseStateLocked() {
	if c.pauseStateFile == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.pauseStateFile), 0o700); err != nil {
		log.Printf("pause state dir: %v", err)
		return
	}
	content := []byte("0")
	if c.paused {
		content = []byte("1")
	}
	if err := os.WriteFile(c.pauseStateFile, content, 0o600); err != nil {
		log.Printf("persist pause state: %v", err)
	}
}

// RestorePauseState loads a persisted pause flag (missing file = running).
// It reports the restored pause state.
func (c *Capturer) RestorePauseState(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	paused := strings.TrimSpace(string(data)) == "1"

	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused
	c.pauseStateFile = path
	c.persistPauseStateLocked() // keep the file normalized ("0" vs "1")
	if paused {
		log.Printf("Restored paused capture state from %s", path)
	}
	return paused
}
