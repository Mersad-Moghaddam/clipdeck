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
	"github.com/Mersad-Moghaddam/clipdeck/internal/ipc"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
)

var (
	profileFlag   = flag.String("profile", "", "Profile to run under ('dev' or empty for default)")
	noCaptureFlag = flag.Bool("no-capture", false, "Disable clipboard capture daemon (IPC and DB only)")
	configFlag    = flag.String("config", "", "Path to custom config file")
	versionFlag   = flag.Bool("version", false, "Print version and exit")
)

const version = "0.1.0"

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("clipdeckd v%s\n", version)
		os.Exit(0)
	}

	log.Printf("Starting clipdeckd v%s (profile: %q)", version, *profileFlag)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var srv *ipc.Server
	var cap *capture.Capturer

	cap = capture.New(cfg, st, func(event string, item *store.Item) {
		if srv != nil {
			srv.Broadcast(event, item)
		}
	})

	srv = ipc.NewServer(cfg, st, cap)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start IPC server: %v", err)
	}
	defer srv.Stop()

	log.Printf("IPC socket listening at %s", cfg.IPC.SocketPath)

	if !*noCaptureFlag {
		log.Printf("Starting clipboard watcher...")
		if err := cap.Start(ctx); err != nil {
			log.Printf("Warning: capture watcher failed to start: %v", err)
		}
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
