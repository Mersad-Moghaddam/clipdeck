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

func TestParseGlobalArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantProfile string
		wantConfig  string
		wantSubcmd  string
		wantSubArgs []string
		wantErr     bool
	}{
		{
			name:        "subcommand with flags",
			args:        []string{"list", "--json"},
			wantSubcmd:  "list",
			wantSubArgs: []string{"--json"},
		},
		{
			name:        "global flag before subcommand with flags",
			args:        []string{"--profile", "dev", "list", "--type", "code"},
			wantProfile: "dev",
			wantSubcmd:  "list",
			wantSubArgs: []string{"--type", "code"},
		},
		{
			name:        "copy with trailing plain flag",
			args:        []string{"copy", "item-123", "--plain"},
			wantSubcmd:  "copy",
			wantSubArgs: []string{"item-123", "--plain"},
		},
		{
			name:        "pick with tool flag",
			args:        []string{"pick", "--tool", "fzf", "--paste"},
			wantSubcmd:  "pick",
			wantSubArgs: []string{"--tool", "fzf", "--paste"},
		},
		{
			name:    "unknown global flag before subcommand",
			args:    []string{"--unknown-flag", "list"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prof, cfg, subcmd, subArgs, err := parseGlobalArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if tc.wantErr {
				return
			}
			if prof != tc.wantProfile {
				t.Errorf("profile: got %q, want %q", prof, tc.wantProfile)
			}
			if cfg != tc.wantConfig {
				t.Errorf("config: got %q, want %q", cfg, tc.wantConfig)
			}
			if subcmd != tc.wantSubcmd {
				t.Errorf("subcmd: got %q, want %q", subcmd, tc.wantSubcmd)
			}
			if len(subArgs) != len(tc.wantSubArgs) {
				t.Fatalf("subArgs len: got %v, want %v", subArgs, tc.wantSubArgs)
			}
			for i := range subArgs {
				if subArgs[i] != tc.wantSubArgs[i] {
					t.Errorf("subArgs[%d]: got %q, want %q", i, subArgs[i], tc.wantSubArgs[i])
				}
			}
		})
	}
}
