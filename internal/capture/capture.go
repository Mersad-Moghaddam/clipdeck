package capture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Mersad-Moghaddam/clipdeck/internal/config"
	"github.com/Mersad-Moghaddam/clipdeck/internal/detect"
	"github.com/Mersad-Moghaddam/clipdeck/internal/hypr"
	"github.com/Mersad-Moghaddam/clipdeck/internal/sensitive"
	"github.com/Mersad-Moghaddam/clipdeck/internal/store"
	"github.com/Mersad-Moghaddam/clipdeck/internal/thumb"
)

type Capturer struct {
	cfg     *config.Config
	store   *store.Store
	paused  bool
	onEvent func(event string, item *store.Item)
	mu      sync.RWMutex
}

func New(cfg *config.Config, s *store.Store, onEvent func(string, *store.Item)) *Capturer {
	return &Capturer{
		cfg:     cfg,
		store:   s,
		onEvent: onEvent,
	}
}

func (c *Capturer) SetPaused(paused bool) {
	c.mu.Lock()
	c.paused = paused
	c.mu.Unlock()
}

func (c *Capturer) IsPaused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paused
}

func (c *Capturer) Start(ctx context.Context) error {
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

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var cmd *exec.Cmd
		if isImage {
			// Watch images
			cmd = exec.CommandContext(ctx, "wl-paste", "--type", "image", "--watch", "cat")
		} else {
			// Watch text
			cmd = exec.CommandContext(ctx, "wl-paste", "--type", "text", "--watch", "cat")
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(backoff)
			continue
		}

		if err := cmd.Start(); err != nil {
			time.Sleep(backoff)
			continue
		}

		// When wl-paste --watch fires, it spawns its child with the payload on stdin.
		// However, in Go we can also poll or use a helper child process.
		// Let's implement SnapshotCurrent() as the reliable ingest method triggered on change!
		_ = stdout
		_ = cmd.Wait()

		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * 1.5)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
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

	var expiresAt int64 = 0
	if sensCheck.IsSensitive {
		expiresAt = time.Now().Unix() + int64(c.cfg.Privacy.SensitiveTTLSec)
	}

	preview := det.Preview
	if sensCheck.IsSensitive {
		preview = sensCheck.MaskedText
	}

	item := &store.Item{
		Hash:          hash,
		Type:          det.Type,
		Mime:          "text/plain",
		Content:       text,
		Preview:       preview,
		RichPreview:   det.RichPreview,
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
	if strings.Contains(mime, "jpeg") || strings.Contains(mime, "jpg") {
		ext = "jpg"
	} else if strings.Contains(mime, "webp") {
		ext = "webp"
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
	// Query current types from wl-paste
	cmd := exec.Command("wl-paste", "--list-types")
	out, err := cmd.Output()
	if err != nil {
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

	// Check URI list (files)
	if typeMap["text/uri-list"] {
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
