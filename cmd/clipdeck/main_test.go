package main

import (
	"encoding/json"
	"testing"
)

// The synthetic seed JSON must be valid JSON: it used to be a backtick raw
// string full of literal "\n" sequences (initial double backslash), which
// produced a broken pseudo-JSON item seeded into the profile's store.
func TestDevSeedJSONIsValid(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(devSeedJSON), &parsed); err != nil {
		t.Fatalf("dev seed JSON is invalid: %v", err)
	}
	if parsed["name"] != "omarchy-clipdeck" {
		t.Fatalf("unexpected seed JSON content: %v", parsed)
	}
}

// Seeding may only ever run on the dev profile so it can never touch the
// user's real clipboard history.
func TestDevSeedProfileGuard(t *testing.T) {
	if !isDevSeedProfile("dev") {
		t.Fatal("dev profile must be allowed to seed")
	}
	for _, p := range []string{"", "dev-with-suffix", "DEV"} {
		if isDevSeedProfile(p) {
			t.Fatalf("profile %q must not be allowed to seed", p)
		}
	}
}
