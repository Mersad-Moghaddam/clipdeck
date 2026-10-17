package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Storage StorageConfig `toml:"storage"`
	Capture CaptureConfig `toml:"capture"`
	Privacy PrivacyConfig `toml:"privacy"`
	IPC     IPCConfig     `toml:"ipc"`
}

type StorageConfig struct {
	DataDir      string `toml:"data_dir"`
	MaxItems     int    `toml:"max_items"`
	MaxAgeDays   int    `toml:"max_age_days"`
	MaxSizeBytes int64  `toml:"max_size_bytes"`
}

type CaptureConfig struct {
	MaxTextBytes  int64    `toml:"max_text_bytes"`
	MaxImageBytes int64    `toml:"max_image_bytes"`
	ExcludedApps  []string `toml:"excluded_apps"`

	// DebounceMs is the echo-suppression window: duplicate ingests of the
	// same content within the window are dropped instead of resaving and
	// rebroadcasting (kill switch for the text+image watcher double fire).
	DebounceMs int `toml:"debounce_ms"` // floored at 250 at runtime
}

type PrivacyConfig struct {
	// AutoMaskSensitive is currently reserved: masking is always-on for the
	// detectors below (removing the flag would change privacy behavior).
	AutoMaskSensitive bool `toml:"auto_mask_sensitive"`
	SensitiveTTLSec   int  `toml:"sensitive_ttl_sec"`

	// The detector toggles are still being wired (v0.2.3); today every
	// detector runs unconditionally. Reserved flags are accepted so configs
	// keep validating.
	CheckLuhn        bool `toml:"check_luhn"`        // reserved, always-on
	CheckAPIKeys     bool `toml:"check_api_keys"`    // reserved, always-on
	CheckPrivateKeys bool `toml:"check_private_keys"` // reserved, always-on

	// FetchLinkTitles stays strictly false by default (PRD §9) and is not
	// implemented yet; the daemon never performs network fetches.
	FetchLinkTitles bool `toml:"fetch_link_titles"`
}

type IPCConfig struct {
	SocketPath string `toml:"socket_path"`
}

func DefaultConfig(profile string) *Config {
	homeDir, _ := os.UserHomeDir()
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), "run-clipdeck")
	}

	dataSubDir := "clipdeck"
	socketSubDir := "clipdeck"
	if profile == "dev" {
		dataSubDir = "clipdeck-dev"
		socketSubDir = "clipdeck-dev"
	}

	dataDir := filepath.Join(homeDir, ".local", "share", dataSubDir)
	socketPath := filepath.Join(runtimeDir, socketSubDir, "clipdeck.sock")

	return &Config{
		Storage: StorageConfig{
			DataDir:      dataDir,
			MaxItems:     10000,
			MaxAgeDays:   30,
			MaxSizeBytes: 100 * 1024 * 1024, // 100 MB
		},
		Capture: CaptureConfig{
			MaxTextBytes:  5 * 1024 * 1024,  // 5 MB
			MaxImageBytes: 20 * 1024 * 1024, // 20 MB
			ExcludedApps: []string{
				"1Password",
				"keepassxc",
				"bitwarden",
				"org.keepassxc.KeePassXC",
				"com.bitwarden.desktop",
			},
			DebounceMs: 150,
		},
		Privacy: PrivacyConfig{
			AutoMaskSensitive: true,
			SensitiveTTLSec:   60,
			CheckLuhn:         true,
			CheckAPIKeys:      true,
			CheckPrivateKeys:  true,
			FetchLinkTitles:   false, // strictly false by default (PRD §9)
		},
		IPC: IPCConfig{
			SocketPath: socketPath,
		},
	}
}

func LoadConfig(path string, profile string) (*Config, error) {
	cfg := DefaultConfig(profile)

	if path == "" {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, ".config", "clipdeck", "config.toml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return cfg, err // return defaults with parse error
	}

	return cfg, nil
}
