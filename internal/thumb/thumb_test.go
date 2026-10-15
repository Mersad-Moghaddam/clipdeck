package thumb

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestGenerateThumbnail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "clipdeck-thumb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create synthetic 400x300 image
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}

	info, err := GenerateThumbnail(buf.Bytes(), tmpDir, 200)
	if err != nil {
		t.Fatalf("GenerateThumbnail: %v", err)
	}

	if info.Width != 400 || info.Height != 300 {
		t.Fatalf("Expected dimensions 400x300, got %dx%d", info.Width, info.Height)
	}

	if _, err := os.Stat(info.ThumbnailPath); err != nil {
		t.Fatalf("Thumbnail file missing: %v", err)
	}
}
