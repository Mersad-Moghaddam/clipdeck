package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
)

func TestDoctorRunsWithoutCrashing(t *testing.T) {
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
