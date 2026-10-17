package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/detect"
	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
	"github.com/Mersad-Moghaddam/clipdeck/internal/thumb"
)

var (
	profileFlag = flag.String("profile", "", "Profile to run under ('dev' or empty for default)")
	configFlag  = flag.String("config", "", "Path to custom config file")
)

var version = "0.3.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Extract global flags if placed before subcommand
	args := os.Args[1:]
	var subcmd string
	var subcmdArgs []string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--profile" || args[i] == "-profile") && i+1 < len(args) {
			*profileFlag = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--profile=") || strings.HasPrefix(args[i], "-profile=") {
			*profileFlag = strings.TrimPrefix(strings.TrimPrefix(args[i], "--profile="), "-profile=")
		} else if (args[i] == "--config" || args[i] == "-config") && i+1 < len(args) {
			*configFlag = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--config=") || strings.HasPrefix(args[i], "-config=") {
			*configFlag = strings.TrimPrefix(strings.TrimPrefix(args[i], "--config="), "-config=")
		} else if strings.HasPrefix(args[i], "-") {
			// Unknown flag: fail loudly instead of silently swallowing it —
			// users must be able to trust CLI exit codes.
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\nUsage: clipdeck [--profile dev] [--config file] <command>\n", args[i])
			os.Exit(1)
		} else if subcmd == "" {
			subcmd = args[i]
			subcmdArgs = args[i+1:]
		}
	}

	if subcmd == "" {
		printUsage()
		os.Exit(1)
	}

	cfg := config.DefaultConfig(*profileFlag)
	if *configFlag != "" {
		loadedCfg, err := config.LoadConfig(*configFlag, *profileFlag)
		if err != nil {
			// Don't silently fall back to defaults — a malformed config must
			// never pretend to be a working one.
			fmt.Fprintf(os.Stderr, "Warning: config %s is malformed (%v); using defaults\n", *configFlag, err)
		} else {
			cfg = loadedCfg
		}
	}

	switch subcmd {
	case "status":
		runStatus(cfg)
	case "doctor":
		runDoctor(cfg)
	case "dev":
		runDev(cfg, subcmdArgs)
	case "clear":
		runClear(cfg, subcmdArgs)
	case "export":
		runExport(cfg, subcmdArgs)
	case "import":
		runImport(cfg, subcmdArgs)
	case "import-cliphist":
		runImportCliphist(cfg)
	case "pause":
		runPause(cfg, subcmdArgs)
	case "version", "--version", "-v":
		fmt.Printf("clipdeck v%s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`ClipDeck CLI v%s — Keyboard-first clipboard manager

Usage:
  clipdeck [--profile dev] <command> [arguments]

Commands:
  status             Show daemon and store status
  doctor             Run diagnostic checks on system dependencies and environment
  clear              Clear clipboard history (--include-pinned to clear pins too)
  export             Export clipboard history as JSON (--output <file>)
  import <file>      Import clipboard history from JSON file
  import-cliphist    Import existing history from cliphist
  pause [on|off]     Get or set capture pause state
  dev seed           Populate database with synthetic test items (dev profile recommended)
  version            Show version
`, version)
}

func runStatus(cfg *config.Config) {
	client, err := ipc.NewClient(cfg.IPC.SocketPath)
	if err != nil {
		fmt.Printf("● Daemon: Offline (could not connect to %s)\n", cfg.IPC.SocketPath)
		// Check direct store
		if st, sErr := store.Open(cfg.Storage.DataDir); sErr == nil {
			defer st.Close()
			total, pinned, size, _ := st.GetStats()
			fmt.Printf("● Store (offline check): %d items (%d pinned), %.2f MB on disk\n",
				total, pinned, float64(size)/(1024*1024))
		}
		return
	}
	defer client.Close()

	var status struct {
		Version       string `json:"version"`
		Paused        bool   `json:"paused"`
		TotalItems    int    `json:"total_items"`
		PinnedItems   int    `json:"pinned_items"`
		DBSizeBytes   int64  `json:"db_size_bytes"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}

	if err := client.Call("status", nil, &status); err != nil {
		fmt.Printf("Failed to get status from daemon: %v\n", err)
		return
	}

	state := "Active"
	if status.Paused {
		state = "Paused (Incognito)"
	}

	fmt.Printf("● ClipDeck Daemon: v%s (%s)\n", status.Version, state)
	fmt.Printf("  Uptime:       %s\n", time.Duration(status.UptimeSeconds)*time.Second)
	fmt.Printf("  Socket:       %s\n", cfg.IPC.SocketPath)
	fmt.Printf("  Items:        %d total, %d pinned\n", status.TotalItems, status.PinnedItems)
	fmt.Printf("  Storage Size: %.2f MB\n", float64(status.DBSizeBytes)/(1024*1024))
}

func runDoctor(cfg *config.Config) {
	fmt.Printf("ClipDeck Doctor — Diagnostic Suite\n")
	fmt.Printf("==================================\n\n")

	type checkItem struct {
		Name     string
		Required bool
		Passed   bool
		Message  string
	}

	var checks []checkItem

	// 1. Dependency checks
	binaries := []struct {
		bin      string
		required bool
		desc     string
	}{
		{"wl-paste", true, "Wayland clipboard monitor and reader"},
		{"wl-copy", true, "Wayland clipboard writer"},
		{"wtype", false, "Virtual keyboard direct-paste tool"},
		{"hyprctl", false, "Hyprland IPC client"},
		{"quickshell", false, "Omarchy Quattro shell engine"},
	}

	for _, b := range binaries {
		path, err := exec.LookPath(b.bin)
		passed := err == nil
		msg := "found at " + path
		if !passed {
			if b.required {
				msg = "MISSING (Required for operation)"
			} else {
				msg = "optional, not found"
			}
		}
		checks = append(checks, checkItem{
			Name:     b.bin,
			Required: b.required,
			Passed:   passed,
			Message:  msg,
		})
	}

	// 2. Directory & Permission checks
	dirs := []struct {
		path     string
		required bool
		name     string
	}{
		{cfg.Storage.DataDir, true, "Data Dir: " + cfg.Storage.DataDir},
		{filepath.Dir(cfg.IPC.SocketPath), false, "Socket Dir: " + filepath.Dir(cfg.IPC.SocketPath)},
	}
	for _, d := range dirs {
		stat, err := os.Stat(d.path)
		if err != nil {
			if d.required {
				// Try creating it
				if mkErr := os.MkdirAll(d.path, 0700); mkErr == nil {
					checks = append(checks, checkItem{
						Name:     d.name,
						Required: d.required,
						Passed:   true,
						Message:  "created (permissions: 0700)",
					})
					continue
				}
			}
			checks = append(checks, checkItem{
				Name:     d.name,
				Required: d.required,
				Passed:   !d.required,
				Message:  "not created yet (created on daemon start)",
			})
		} else {
			perm := stat.Mode().Perm()
			passed := perm&0077 == 0 // verify no group/other access
			msg := fmt.Sprintf("exists (permissions: %04o)", perm)
			if !passed {
				msg += " WARNING: permissions too open (should be 0700)"
			}
			checks = append(checks, checkItem{
				Name:     d.name,
				Required: d.required,
				Passed:   passed,
				Message:  msg,
			})
		}
	}

	// 3. Daemon IPC check
	client, err := ipc.NewClient(cfg.IPC.SocketPath)
	if err != nil {
		checks = append(checks, checkItem{
			Name:     "Daemon IPC (" + cfg.IPC.SocketPath + ")",
			Required: false,
			Passed:   false,
			Message:  "daemon offline or socket unreachable",
		})
	} else {
		_ = client.Close()
		checks = append(checks, checkItem{
			Name:     "Daemon IPC (" + cfg.IPC.SocketPath + ")",
			Required: false,
			Passed:   true,
			Message:  "connected successfully",
		})
	}

	// 4. Database integrity check
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		checks = append(checks, checkItem{
			Name:     "Database Integrity",
			Required: true,
			Passed:   false,
			Message:  "open failed: " + err.Error(),
		})
	} else {
		total, pinned, size, _ := st.GetStats()
		st.Close()
		checks = append(checks, checkItem{
			Name:     "Database Integrity",
			Required: true,
			Passed:   true,
			Message:  fmt.Sprintf("OK (%d items, %d pinned, %d bytes)", total, pinned, size),
		})
	}

	// Print results
	allPassed := true
	for _, c := range checks {
		statusStr := "[ PASS ]"
		if !c.Passed {
			if c.Required {
				statusStr = "[ FAIL ]"
				allPassed = false
			} else {
				statusStr = "[ WARN ]"
			}
		}
		fmt.Printf("%-8s %-30s %s\n", statusStr, c.Name, c.Message)
	}

	fmt.Println()
	if allPassed {
		fmt.Println("Result: All required checks passed successfully.")
	} else {
		fmt.Println("Result: Some required checks failed. Please review above.")
		os.Exit(1)
	}
}

// devSeedJSON must stay a raw literal with real newlines — an escaped
// "\n"-in-backticks variant once seeded broken pseudo-JSON items.
const devSeedJSON = `{
  "name": "omarchy-clipdeck",
  "version": "0.3.0",
  "status": "production-ready",
  "stars": 128
}`

// isDevSeedProfile is the single gate: without an explicit dev profile the
// seed must refuse to run, since it writes into the configured data dir.
func isDevSeedProfile(profile string) bool {
	return profile == "dev"
}

func runDev(cfg *config.Config, args []string) {
	if len(args) == 0 || args[0] != "seed" {
		fmt.Println("Usage: clipdeck dev seed")
		return
	}

	if !isDevSeedProfile(*profileFlag) {
		fmt.Fprintln(os.Stderr, "Refusing to seed: `clipdeck dev seed` only runs with `--profile dev` to protect your real history.")
		os.Exit(1)
	}

	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	fmt.Println("Seeding synthetic data into", cfg.Storage.DataDir, "...")

	now := time.Now()

	// 1. Plain text
	saveSynthetic(st, "Hello ClipDeck! Welcome to Omarchy Quattro clipboard manager.", "kitty", false, now.Add(-10*time.Minute))

	// 2. Go code
	goCode := `package main

import "fmt"

func main() {
	fmt.Println("Hello Omarchy!")
}`
	saveSynthetic(st, goCode, "code", true, now.Add(-9*time.Minute))

	// 3. Python code
	pyCode := `def fibonacci(n: int) -> int:
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print([fibonacci(i) for i in range(10)])`
	saveSynthetic(st, pyCode, "kitty", false, now.Add(-8*time.Minute))

	// 4. JSON
	saveSynthetic(st, devSeedJSON, "firefox", false, now.Add(-7*time.Minute))

	// 5. Color hex & rgb
	saveSynthetic(st, "#e06c75", "gcolor3", true, now.Add(-6*time.Minute))
	saveSynthetic(st, "rgb(97, 175, 239)", "gcolor3", false, now.Add(-5*time.Minute))

	// 6. URL / Link
	saveSynthetic(st, "https://github.com/Mersad-Moghaddam/clipdeck", "firefox", false, now.Add(-4*time.Minute))
	saveSynthetic(st, "https://omarchy.org/docs/quattro/plugins", "firefox", false, now.Add(-3*time.Minute))

	// 7. Persian text with mixed Latin & Persian digits
	persianText := "سلام به دنیای امارچی ۴! این یک متن تستی فارسی با ارقام ۱۲۳۴۵ و حروف عربي ي و ك است."
	saveSynthetic(st, persianText, "telegram-desktop", false, now.Add(-2*time.Minute))

	// 8. Persian poem (RTL)
	poem := `بنی آدم اعضای یک پیکرند
که در آفرینش ز یک گوهرند
چو عضوی به درد آورد روزگار
دگر عضوها را نماند قرار`
	saveSynthetic(st, poem, "telegram-desktop", true, now.Add(-1*time.Minute))

	// 9. Email & Phone
	saveSynthetic(st, "contact@mersad.dev", "thunderbird", false, now.Add(-45*time.Second))
	saveSynthetic(st, "+98 912 345 6789", "telegram-desktop", false, now.Add(-30*time.Second))

	// 10. File path
	saveSynthetic(st, "/home/mersad/.config/hypr/hyprland.conf", "kitty", false, now.Add(-20*time.Second))

	// 11. Synthetic Image
	seedImage(st, cfg.Storage.DataDir, now.Add(-10*time.Second))

	total, pinned, _, _ := st.GetStats()
	fmt.Printf("Seed complete! Store now contains %d items (%d pinned).\n", total, pinned)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func sha256BytesHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func saveSynthetic(st *store.Store, text, app string, pinned bool, t time.Time) {
	d := detect.Detect(text)
	preview := d.Preview
	if preview == "" {
		preview = text
		if len(preview) > 100 {
			preview = preview[:100] + "…"
		}
	}

	mime := "text/plain"
	if d.Type == "json" {
		mime = "application/json"
	}

	item := &store.Item{
		Hash:        sha256Hex(text),
		Type:        d.Type,
		Mime:        mime,
		Content:     text,
		Preview:     preview,
		RichPreview: d.RichPreview,
		SourceApp:   app,
		Pinned:      pinned,
		CreatedAt:   t,
		UpdatedAt:   t,
		SizeBytes:   int64(len(text)),
	}
	if pinned {
		item.PinnedAt = t.Unix()
	}
	_ = st.SaveItem(item)
}

func seedImage(st *store.Store, dataDir string, t time.Time) {
	// Create a 128x128 sample image
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	accent := color.RGBA{R: 97, G: 175, B: 239, A: 255}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if (x-64)*(x-64)+(y-64)*(y-64) < 40*40 {
				img.Set(x, y, accent)
			} else {
				img.Set(x, y, color.RGBA{R: 40, G: 44, B: 52, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	data := buf.Bytes()

	blobPath, err := st.SaveBlob(data, "png")
	if err != nil {
		return
	}

	thumbsDir := filepath.Join(dataDir, "thumbs")
	thumbInfo, err := thumb.GenerateThumbnail(data, thumbsDir, 240)
	var thumbPath string
	if err == nil && thumbInfo != nil {
		thumbPath = thumbInfo.ThumbnailPath
	}

	item := &store.Item{
		Hash:          sha256BytesHex(data),
		Type:          "image",
		Mime:          "image/png",
		Content:       "[PNG Image 128x128]",
		Preview:       "[PNG Image 128x128]",
		BlobPath:      blobPath,
		ThumbnailPath: thumbPath,
		SourceApp:     "swappy",
		Pinned:        false,
		CreatedAt:     t,
		UpdatedAt:     t,
		SizeBytes:     int64(len(data)),
	}
	_ = st.SaveItem(item)
}

func runClear(cfg *config.Config, args []string) {
	includePinned := false
	for _, a := range args {
		if a == "--include-pinned" {
			includePinned = true
		}
	}

	client, err := ipc.NewClient(cfg.IPC.SocketPath)
	if err == nil {
		defer client.Close()
		var res map[string]any
		if err := client.Call("clear", map[string]any{"include_pinned": includePinned}, &res); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to clear via daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cleared history successfully (deleted %v items).\n", res["deleted"])
		return
	}

	// Offline direct store clear
	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	deleted, err := st.Clear(includePinned)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clear store: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cleared history directly from store (deleted %d items).\n", deleted)
}

func runExport(cfg *config.Config, args []string) {
	var outputFile string
	for i := 0; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			outputFile = args[i+1]
			i++
		}
	}

	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	items, _, err := st.ListItems(100000, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query items: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode items: %v\n", err)
		os.Exit(1)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported %d items to %s\n", len(items), outputFile)
	} else {
		os.Stdout.Write(data)
	}
}

func runImport(cfg *config.Config, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: clipdeck import <file.json>\n")
		os.Exit(1)
	}

	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read %s: %v\n", filePath, err)
		os.Exit(1)
	}

	var items []store.Item
	if err := json.Unmarshal(data, &items); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse JSON: %v\n", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	imported := 0
	for _, item := range items {
		// Exports are untrusted: neutralize arbitrary blob paths so future
		// deletes can never act on foreign files.
		st.SanitizeImportedItem(&item)
		if err := st.SaveItem(&item); err == nil {
			imported++
		}
	}

	fmt.Printf("Imported %d of %d items into %s\n", imported, len(items), cfg.Storage.DataDir)
}

func runImportCliphist(cfg *config.Config) {
	cliphistBin, err := exec.LookPath("cliphist")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cliphist is not installed on this system\n")
		os.Exit(1)
	}

	cmd := exec.Command(cliphistBin, "list")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run cliphist list: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(out), "\n")
	fmt.Printf("Found %d cliphist entries. Importing...\n", len(lines))

	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	imported := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// cliphist format is: <id>\t<content preview>
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		decodeCmd := exec.Command(cliphistBin, "decode", parts[0])
		contentBytes, err := decodeCmd.Output()
		if err != nil {
			continue
		}
		content := string(contentBytes)
		d := detect.Detect(content)
		preview := d.Preview
		if preview == "" {
			preview = content
			if len(preview) > 100 {
				preview = preview[:100] + "…"
			}
		}
		item := &store.Item{
			Hash:        sha256Hex(content),
			Type:        d.Type,
			Mime:        "text/plain",
			Content:     content,
			Preview:     preview,
			RichPreview: d.RichPreview,
			SourceApp:   "cliphist",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			SizeBytes:   int64(len(content)),
		}
		if err := st.SaveItem(item); err == nil {
			imported++
		}
	}

	fmt.Printf("Successfully imported %d items from cliphist!\n", imported)
}

func runPause(cfg *config.Config, args []string) {
	client, err := ipc.NewClient(cfg.IPC.SocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Daemon offline: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if len(args) == 0 {
		var status struct {
			Paused bool `json:"paused"`
		}
		_ = client.Call("status", nil, &status)
		if status.Paused {
			fmt.Println("ClipDeck is currently PAUSED (incognito)")
		} else {
			fmt.Println("ClipDeck is currently ACTIVE")
		}
		return
	}

	target := strings.ToLower(args[0])
	setPause := false
	if target == "on" || target == "true" || target == "pause" {
		setPause = true
	} else if target == "toggle" {
		var status struct {
			Paused bool `json:"paused"`
		}
		_ = client.Call("status", nil, &status)
		setPause = !status.Paused
	}

	var res map[string]any
	if err := client.Call("pause", map[string]any{"paused": setPause}, &res); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set pause: %v\n", err)
		os.Exit(1)
	}

	if setPause {
		fmt.Println("ClipDeck capture PAUSED (incognito mode enabled)")
	} else {
		fmt.Println("ClipDeck capture RESUMED (incognito mode disabled)")
	}
}
