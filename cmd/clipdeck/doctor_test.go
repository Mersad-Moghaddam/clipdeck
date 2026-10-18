package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
)

func TestDoctorRunsWithoutCrashing(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	cfg := config.DefaultConfig("dev")
	cfg.Storage.DataDir = t.TempDir()

	var buf bytes.Buffer
	_ = runDoctor(cfg, &buf)
	output := buf.String()

	requiredSubstrings := []string{
		"ClipDeck Doctor",
		"wl-paste",
		"Database Integrity",
		"Session / Wayland",
		"Source App Attribution",
	}
	for _, s := range requiredSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("doctor output missing %q:\n%s", s, output)
		}
	}
}

func TestDoctorX11DoesNotRequireWayland(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("DISPLAY", ":99")
	t.Setenv("WAYLAND_DISPLAY", "")
	cfg := config.DefaultConfig("dev")
	cfg.Storage.DataDir = t.TempDir()
	var buf bytes.Buffer
	runDoctor(cfg, &buf)
	output := buf.String()
	if !strings.Contains(output, "Session / X11") || !strings.Contains(output, "xclip") {
		t.Fatalf("X11 diagnostics missing: %s", output)
	}
	if strings.Contains(output, "wl-paste") {
		t.Fatalf("Wayland dependency required on X11: %s", output)
	}
}

func TestDoctorRejectsGNOMEWaylandCapture(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_CURRENT_DESKTOP", "zorin:GNOME")
	cfg := config.DefaultConfig("dev")
	cfg.Storage.DataDir = t.TempDir()
	var buf bytes.Buffer
	passed := runDoctor(cfg, &buf)
	if passed || !strings.Contains(buf.String(), "GNOME Wayland clipboard capture is unsupported") {
		t.Fatalf("expected actionable capture failure, got %v: %s", passed, buf.String())
	}
}
