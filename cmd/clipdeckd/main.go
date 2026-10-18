package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/capture"
	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/detect"
	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

var (
	profileFlag   = flag.String("profile", "", "Profile to run under ('dev' or empty for default)")
	noCaptureFlag = flag.Bool("no-capture", false, "Disable clipboard capture daemon (IPC and DB only)")
	configFlag    = flag.String("config", "", "Path to custom config file")
	versionFlag   = flag.Bool("version", false, "Print version and exit")
)

var version = "0.4.2"

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("clipdeckd v%s\n", version)
		os.Exit(0)
	}

	log.Printf("Starting clipdeckd v%s (profile: %q)", version, *profileFlag)
	ipc.Version = version

	cfg := config.DefaultConfig(*profileFlag)
	if *configFlag != "" {
		loadedCfg, err := config.LoadConfig(*configFlag, *profileFlag)
		if err != nil {
			log.Fatalf("Failed to load config from %s: %v", *configFlag, err)
		}
		cfg = loadedCfg
	}

	st, err := store.Open(cfg.Storage.DataDir)
	if err != nil {
		log.Fatalf("Failed to open store at %s: %v", cfg.Storage.DataDir, err)
	}
	defer st.Close()

	// Expire sensitive history synchronously before serving anything: an
	// expired token must not be readable by the first IPC client ahead of the
	// 30s maintenance tick.
	if purged, err := st.PurgeExpired(); err == nil && purged > 0 {
		log.Printf("Startup purge removed %d expired sensitive items", purged)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Re-render stored code/JSON previews that predate the theme-agnostic
	// markup format. Runs in the background so a cold start never blocks on
	// history size.
	go refreshLegacyMarkup(ctx, st)

	var srv *ipc.Server
	var cap *capture.Capturer

	cap = capture.New(cfg, st, func(event string, item *store.Item) {
		if srv != nil {
			srv.Broadcast(event, item)
		}
	})
	// Pause state persists across restarts (production/dev paths isolated).
	cap.SetPauseStateFile(capture.PauseStateFile(*profileFlag))
	if restored := cap.RestorePauseState(capture.PauseStateFile(*profileFlag)); restored {
		log.Printf("Capture starts in PAUSED state (persisted)")
	}

	srv = ipc.NewServer(cfg, st, cap)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start IPC server: %v", err)
	}
	defer srv.Stop()

	log.Printf("IPC socket listening at %s", cfg.IPC.SocketPath)

	if !*noCaptureFlag {
		log.Printf("Starting clipboard watcher...")
		// cap.Start blocks until the watchers exit, so it must not run on the
		// main goroutine: SIGTERM handling below would be unreachable and
		// systemd would have to SIGKILL a 90s-stuck stop.
		go func() {
			if err := cap.Start(ctx); err != nil {
				log.Printf("Warning: capture watcher failed to start: %v", err)
			}
		}()
	} else {
		log.Printf("Capture disabled via --no-capture flag")
	}

	// Periodic maintenance goroutine
	go func() {
		purgeTicker := time.NewTicker(30 * time.Second)
		retentionTicker := time.NewTicker(30 * time.Minute)
		defer purgeTicker.Stop()
		defer retentionTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-purgeTicker.C:
				purged, err := st.PurgeExpired()
				if err == nil && purged > 0 {
					log.Printf("Purged %d expired sensitive items", purged)
					srv.Broadcast("items_purged", map[string]any{"count": purged})
				}
			case <-retentionTicker.C:
				if err := st.EnforceRetention(cfg.Storage.MaxItems, cfg.Storage.MaxAgeDays, cfg.Storage.MaxSizeBytes); err != nil {
					log.Printf("Retention policy error: %v", err)
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)
	cancel()
	srv.Stop()
	log.Printf("clipdeckd shut down cleanly.")
}

// refreshLegacyMarkup walks stored code/JSON clips whose rich preview still
// carries the v0.1.0 inline palette and regenerates them with class-based
// markup. It pages in batches and exits as soon as each row is either
// refreshed or no longer highlightable, so it terminates even on failures.
func refreshLegacyMarkup(ctx context.Context, st *store.Store) {
	const batch = 200
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		items, err := st.LegacyRichPreviewItems(batch)
		if err != nil {
			log.Printf("Rich preview refresh failed: %v", err)
			return
		}
		if len(items) == 0 {
			return
		}

		refreshed := 0
		for i := range items {
			res := detect.Detect(items[i].Content)
			markup := res.RichPreview
			highlightable := res.Type == "code" || res.Type == "json"
			// Sensitive clips never carry rich markup (the only sanctioned
			// reveal is an explicit user read of the plain content).
			if !highlightable || items[i].Sensitive {
				markup = "" // nothing to highlight; stop revisiting
			}
			if err := st.RefreshRichPreview(items[i].ID, markup); err != nil {
				log.Printf("Rich preview refresh failed for %s: %v", items[i].ID, err)
				return
			}
			refreshed++
		}

		log.Printf("Re-rendered %d stored rich previews for the active theme", refreshed)
		if len(items) < batch {
			return
		}
	}
}
