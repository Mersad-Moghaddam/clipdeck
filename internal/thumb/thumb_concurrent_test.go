package thumb

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"sync"
	"testing"
)

func renderTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7 % 255), uint8(y * 11 % 255), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// Parallel calls for the SAME image share the hash-named target file; every
// concurrent generation must succeed (no tmp-file rename collisions) and all
// of them must agree on the target path.
func TestGenerateThumbnailParallelCalls(t *testing.T) {
	dir := t.TempDir()
	data := renderTestPNG(t, 320, 240)

	const workers = 8
	results := make([]*ImageInfo, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = GenerateThumbnail(data, dir, 240)
		}(i)
	}
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("worker %d: %v", i, errs[i])
		}
		if results[i] == nil || results[i].ThumbnailPath == "" {
			t.Fatalf("worker %d: no thumbnail path", i)
		}
		if results[i].ThumbnailPath != results[0].ThumbnailPath {
			// All workers hash to the same image: paths must match.
			t.Fatalf("worker %d: inconsistent thumb path %s", i, results[i].ThumbnailPath)
		}
	}

	// The final thumbnail exists and no tmp files leak in the directory.
	if _, err := os.Stat(results[0].ThumbnailPath); err != nil {
		t.Fatalf("thumbnail missing: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("tmp file leaked: %s", e.Name())
		}
	}
}
