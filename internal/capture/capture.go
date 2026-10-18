package capture

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/desktop"
	"github.com/Mersad-Moghaddam/clipdeck/internal/detect"
	"github.com/Mersad-Moghaddam/clipdeck/internal/hypr"
	"github.com/Mersad-Moghaddam/clipdeck/internal/sensitive"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
	"github.com/Mersad-Moghaddam/clipdeck/internal/thumb"
)

type Capturer struct {
	backend desktop.Backend
	cfg     *config.Config
	store   *store.Store
	paused  bool
	onEvent func(event string, item *store.Item)
	mu      sync.RWMutex
	snapMu  sync.Mutex // serializes snapshots between the text and image watchers

	// Echo suppression: the text and image watchers both wake on the same
	// selection change, so identical content can reach the ingest path twice
	// in quick succession. The second ingest must not re-save/re-broadcast.
	lastEchoHash string
	lastEchoAt   time.Time

	// Optional pause-state persistence (paths set by the daemon entrypoint).
	pauseStateFile string
}

func New(cfg *config.Config, s *store.Store, onEvent func(string, *store.Item)) *Capturer {
	var backend desktop.Backend
	if session, err := desktop.DetectSession(os.Getenv); err == nil && session == "x11" {
		backend = desktop.NewX11Backend()
	}
	return &Capturer{
		backend: backend,
		cfg:     cfg,
		store:   s,
		onEvent: onEvent,
	}
}

func (c *Capturer) SetPaused(paused bool) {
	c.mu.Lock()
	c.paused = paused
	c.persistPauseStateLocked()
	c.mu.Unlock()
}

func (c *Capturer) IsPaused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paused
}

// isDuplicateEcho reports whether `hash` was already ingested inside the
// debounce window, and records it otherwise. Call it once per candidate item
// right after hashing: snapshots are cheap to repeat, store writes and
// broadcasts are not. The window floor keeps two watcher events of the same
// selection coalesced even when a snapshot is slow.
func (c *Capturer) isDuplicateEcho(hash string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	windowMs := c.cfg.Capture.DebounceMs
	if windowMs < 250 {
		windowMs = 250
	}

	if c.lastEchoHash == hash && time.Since(c.lastEchoAt) < time.Duration(windowMs)*time.Millisecond {
		return true
	}
	c.lastEchoHash = hash
	c.lastEchoAt = time.Now()
	return false
}

func (c *Capturer) Start(ctx context.Context) error {
	if c.backend != nil {
		return c.backend.Watch(ctx, func() {
			c.snapMu.Lock()
			defer c.snapMu.Unlock()
			if _, err := c.SnapshotCurrent(); err != nil {
				log.Printf("capture: %v", err)
			}
		})
	}
	var wg sync.WaitGroup
	wg.Add(2)

	// Watcher 1: text watcher
	go func() {
		defer wg.Done()
		c.runWatcher(ctx, false)
	}()

	// Watcher 2: image watcher
	go func() {
		defer wg.Done()
		c.runWatcher(ctx, true)
	}()

	wg.Wait()
	return nil
}

func (c *Capturer) runWatcher(ctx context.Context, isImage bool) {
	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second

	// wl-paste --watch re-runs this command for every selection change. The
	// command drains the payload and prints exactly one newline per change,
	// turning stdout into a clean one-event-per-line trigger stream. The
	// selection itself is then read via SnapshotCurrent(), which sidesteps
	// payload framing problems (clipboard data has no inherent delimiter).
	watchCommand := "cat >/dev/null; printf '\\n'"
	watcherType := "text"
	if isImage {
		watcherType = "image"
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cmd := exec.CommandContext(ctx, "wl-paste", "--type", watcherType, "--watch", "sh", "-c", watchCommand)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			if !retryWait(ctx, backoff) {
				return
			}
			continue
		}

		if err := cmd.Start(); err != nil {
			if !retryWait(ctx, backoff) {
				return
			}
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			backoff = 500 * time.Millisecond
			// One line per selection change: snapshot the current clipboard
			// and store it. Serialized so the text and image watchers cannot
			// race each other into the store.
			c.snapMu.Lock()
			_, err := c.SnapshotCurrent()
			c.snapMu.Unlock()
			if err != nil {
				log.Printf("capture (%s): %v", watcherType, err)
			}
		}

		_ = cmd.Wait()

		if !retryWait(ctx, backoff) {
			return
		}
		backoff = time.Duration(float64(backoff) * 1.5)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func retryWait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c *Capturer) ProcessTextContent(text string, mimeTypes []string, sourceApp string) (*store.Item, error) {
	if c.IsPaused() {
		return nil, nil
	}

	if int64(len(text)) > c.cfg.Capture.MaxTextBytes {
		return nil, nil // ignore oversized
	}

	if sourceApp == "" {
		win, _ := hypr.GetActiveWindow()
		sourceApp = hypr.SanitizeAppClass(win.Class)
	}

	// Check sensitive
	sensCheck := sensitive.Check(text, mimeTypes, sourceApp, c.cfg.Capture.ExcludedApps)

	// Detect content type
	det := detect.Detect(text)

	hash := sha256Hex(text)

	// Same selection losely-duplicated by both watchers inside the window:
	// skip the redundant save + broadcast.
	if c.isDuplicateEcho(hash) {
		return nil, nil
	}

	var expiresAt int64 = 0
	if sensCheck.IsSensitive {
		expiresAt = time.Now().Unix() + int64(c.cfg.Privacy.SensitiveTTLSec)
	}

	preview := det.Preview
	richPreview := det.RichPreview
	if sensCheck.IsSensitive {
		preview = sensCheck.MaskedText
		// Sensitive code/json must not ship raw content as rich markup; the
		// only sanctioned reveal is an explicit user action.
		richPreview = ""
	}

	item := &store.Item{
		Hash:          hash,
		Type:          det.Type,
		Mime:          "text/plain",
		Content:       text,
		Preview:       preview,
		RichPreview:   richPreview,
		SourceApp:     sourceApp,
		Sensitive:     sensCheck.IsSensitive,
		MaskedContent: sensCheck.MaskedText,
		ExpiresAt:     expiresAt,
		SizeBytes:     int64(len(text)),
	}

	if err := c.store.SaveItem(item); err != nil {
		return nil, err
	}

	if c.onEvent != nil {
		c.onEvent("item_added", item)
	}

	return item, nil
}

func (c *Capturer) ProcessImageContent(data []byte, mime string, sourceApp string) (*store.Item, error) {
	if c.IsPaused() {
		return nil, nil
	}

	if int64(len(data)) > c.cfg.Capture.MaxImageBytes {
		return nil, nil // ignore oversized
	}

	if sourceApp == "" {
		win, _ := hypr.GetActiveWindow()
		sourceApp = hypr.SanitizeAppClass(win.Class)
	}

	ext := "png"
	switch {
	case strings.Contains(mime, "jpeg") || strings.Contains(mime, "jpg"):
		ext = "jpg"
	case strings.Contains(mime, "webp"):
		ext = "webp"
	case strings.Contains(mime, "gif"):
		ext = "gif"
	case strings.Contains(mime, "bmp"):
		ext = "bmp"
	}

	blobPath, err := c.store.SaveBlob(data, ext)
	if err != nil {
		return nil, err
	}

	// Generate thumbnail
	info, err := thumb.GenerateThumbnail(data, c.cfg.Storage.DataDir+"/thumbs", 240)
	var thumbPath string
	dims := "Image"
	if err == nil && info != nil {
		thumbPath = info.ThumbnailPath
		dims = fmt.Sprintf("Image [%d×%d]", info.Width, info.Height)
	}

	hash := sha256BytesHex(data)

	if c.isDuplicateEcho(hash) {
		return nil, nil
	}

	item := &store.Item{
		Hash:          hash,
		Type:          "image",
		Mime:          mime,
		Content:       blobPath,
		Preview:       dims,
		BlobPath:      blobPath,
		ThumbnailPath: thumbPath,
		SourceApp:     sourceApp,
		SizeBytes:     int64(len(data)),
	}

	if err := c.store.SaveItem(item); err != nil {
		return nil, err
	}

	if c.onEvent != nil {
		c.onEvent("item_added", item)
	}

	return item, nil
}

func (c *Capturer) SnapshotCurrent() (*store.Item, error) {
	if c.backend != nil {
		selection, err := c.backend.Read(context.Background())
		if err != nil {
			return nil, err
		}
		if len(selection.Data) == 0 {
			return nil, nil
		}
		if selection.MIME == "text/uri-list" {
			if c.IsPaused() {
				return nil, nil
			}
			var paths []string
			for _, line := range strings.Split(string(selection.Data), "\n") {
				u, err := url.Parse(strings.TrimSpace(line))
				if err == nil && u.Scheme == "file" && (u.Host == "" || u.Host == "localhost") && u.Path != "" {
					paths = append(paths, u.Path)
				}
			}
			if len(paths) == 0 {
				return nil, nil
			}
			content := strings.Join(paths, "\n")
			hash := sha256Hex(content)
			if c.isDuplicateEcho(hash) {
				return nil, nil
			}
			preview := fmt.Sprintf("%d files", len(paths))
			if len(paths) == 1 {
				preview = paths[0]
			}
			win, _ := c.backend.ActiveWindow(context.Background())
			item := &store.Item{Hash: hash, Type: "file", Mime: "text/uri-list", Content: content, Preview: preview, SourceApp: hypr.SanitizeAppClass(win.Class), SizeBytes: int64(len(content))}
			if err := c.store.SaveItem(item); err != nil {
				return nil, err
			}
			if c.onEvent != nil {
				c.onEvent("item_added", item)
			}
			return item, nil
		}
		if strings.HasPrefix(selection.MIME, "image/") {
			return c.ProcessImageContent(selection.Data, selection.MIME, "")
		}
		return c.ProcessTextContent(string(selection.Data), []string{selection.MIME}, "")
	}
	// Query current types from wl-paste
	cmd := exec.Command("wl-paste", "--list-types")
	out, err := cmd.Output()
	if err != nil {
		// wl-paste exits 1 when there is no selection (empty clipboard): a
		// normal transient state, not an error worth a log line per event.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	types := strings.Split(string(out), "\n")
	typeMap := make(map[string]bool)
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t != "" {
			typeMap[t] = true
		}
	}

	// Check for sensitive password manager hint
	if typeMap["x-kde-passwordManagerHint"] || typeMap["application/x-secret-flag"] {
		// Ignore or process as sensitive
		log.Println("Ignored sensitive clipboard content marked by password manager")
		return nil, nil
	}

	// Check image first
	for _, imageMime := range []string{"image/png", "image/jpeg", "image/webp"} {
		if typeMap[imageMime] {
			imgCmd := exec.Command("wl-paste", "--type", imageMime)
			data, err := imgCmd.Output()
			if err == nil && len(data) > 0 {
				return c.ProcessImageContent(data, imageMime, "")
			}
		}
	}

	// Check URI list (files) — but only while capture is running; the inline
	// item build below has no other pause guard.
	if typeMap["text/uri-list"] && !c.IsPaused() {
		fileCmd := exec.Command("wl-paste", "--type", "text/uri-list")
		data, err := fileCmd.Output()
		if err == nil && len(data) > 0 {
			lines := strings.Split(string(data), "\n")
			var validPaths []string
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "file://") {
					u, err := url.Parse(l)
					if err == nil && u.Path != "" {
						validPaths = append(validPaths, u.Path)
					}
				}
			}
			if len(validPaths) > 0 {
				content := strings.Join(validPaths, "\n")
				preview := fmt.Sprintf("%d files", len(validPaths))
				if len(validPaths) == 1 {
					preview = validPaths[0]
				}
				hash := sha256Hex(content)

				if c.isDuplicateEcho(hash) {
					return nil, nil
				}

				win, _ := hypr.GetActiveWindow()
				sourceApp := hypr.SanitizeAppClass(win.Class)

				item := &store.Item{
					Hash:      hash,
					Type:      "file",
					Mime:      "text/uri-list",
					Content:   content,
					Preview:   preview,
					SourceApp: sourceApp,
					SizeBytes: int64(len(content)),
				}
				if err := c.store.SaveItem(item); err != nil {
					return nil, err
				}
				if c.onEvent != nil {
					c.onEvent("item_added", item)
				}
				return item, nil
			}
		}
	}

	// Fallback to text
	textCmd := exec.Command("wl-paste", "--no-newline")
	data, err := textCmd.Output()
	if err == nil && len(data) > 0 {
		return c.ProcessTextContent(string(data), types, "")
	}

	return nil, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func sha256BytesHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
