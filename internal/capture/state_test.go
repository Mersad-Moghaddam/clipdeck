package capture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

// Pause state must survive daemon restarts: pausing writes a state file the
// daemon reloads at startup.
func TestPauseStatePersistence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // isolated state root
	cfg := config.DefaultConfig("")
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	cap := New(cfg, st, nil)

	stateFile := filepath.Join(t.TempDir(), "state", "clipdeck", "paused")
	cap.SetPauseStateFile(stateFile)

	cap.SetPaused(true)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("pause state file not written: %v", err)
	}
	if string(data) != "1" {
		t.Fatalf("expected \"1\" in pause state file, got %q", string(data))
	}

	// A fresh capturer (restart) restores the persisted state.
	reborn := New(cfg, st, nil)
	if !reborn.RestorePauseState(stateFile) {
		t.Fatal("RestorePauseState must report paused=true from the state file")
	}
	if !reborn.IsPaused() {
		t.Fatal("restored capturer must be paused")
	}

	// Unpausing clears the state file for the next daemon.
	cap.SetPaused(false)
	data, err = os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("pause state file must still exist after unpausing: %v", err)
	}
	if string(data) != "0" {
		t.Fatalf("expected \"0\" in pause state file, got %q", string(data))
	}
	reborn2 := New(cfg, st, nil)
	if reborn2.RestorePauseState(stateFile) {
		t.Fatal("RestorePauseState must report false for a cleared state file")
	}
	if reborn2.IsPaused() {
		t.Fatal("restored capturer must not be paused")
	}
}

// A missing state file means "not paused" and must not error.
func TestRestorePauseStateWithoutFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cfg := config.DefaultConfig("")
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	cap := New(cfg, st, nil)
	if cap.RestorePauseState(filepath.Join(t.TempDir(), "missing", "paused")) {
		t.Fatal("missing state file must not restore paused state")
	}
	if cap.IsPaused() {
		t.Fatal("missing state file must leave capture running")
	}
}

// The production/dev state files live in separate directories.
func TestPauseStatePathProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	os.Unsetenv("XDG_STATE_HOME")

	prod := PauseStateFile("")
	dev := PauseStateFile("dev")
	if prod == dev {
		t.Fatalf("prod and dev pause-state files must differ: %s", prod)
	}
	if filepath.Base(prod) != "paused" || filepath.Base(dev) != "paused" {
		t.Fatalf("unexpected pause-state file names: %s / %s", prod, dev)
	}
}
