package thumb

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type ImageInfo struct {
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Format        string `json:"format"`
	ThumbnailPath string `json:"thumbnail_path"`
}

func GenerateThumbnail(data []byte, thumbsDir string, maxDim int) (*ImageInfo, error) {
	if maxDim <= 0 {
		maxDim = 240
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image config: %w", err)
	}

	srcImg, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := srcImg.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	targetW := origW
	targetH := origH

	if origW > maxDim || origH > maxDim {
		if origW > origH {
			targetW = maxDim
			targetH = int(float64(origH) * float64(maxDim) / float64(origW))
		} else {
			targetH = maxDim
			targetW = int(float64(origW) * float64(maxDim) / float64(origH))
		}
	}
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	dstImg := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, bounds, draw.Over, nil)

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	thumbFilename := fmt.Sprintf("%s_thumb.png", hashHex)
	thumbPath := filepath.Join(thumbsDir, thumbFilename)

	if _, err := os.Stat(thumbPath); err == nil {
		return &ImageInfo{
			Width:         cfg.Width,
			Height:        cfg.Height,
			Format:        format,
			ThumbnailPath: thumbPath,
		}, nil
	}

	tmpPath := thumbPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}

	if err := png.Encode(f, dstImg); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return nil, err
	}
	f.Close()

	if err := os.Rename(tmpPath, thumbPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	return &ImageInfo{
		Width:         origW,
		Height:        origH,
		Format:        format,
		ThumbnailPath: thumbPath,
	}, nil
}

// Suppress unused imports
var _ = jpeg.Encode
