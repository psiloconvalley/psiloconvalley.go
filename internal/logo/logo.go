package logo

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/nfnt/resize"
)

const (
	MaxFileSize   = 5 << 20 // 5MB input limit
	MaxLogoHeight = 200     // Output height in pixels
	MaxLogoWidth  = 800     // Output width cap
)

// Process takes raw uploaded bytes, validates the image,
// resizes it to a professional dimension, and returns
// optimized PNG bytes ready for storage.
func Process(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("file too large (max 5MB)")
	}

	// Detect real content type from bytes, not filename
	contentType := http.DetectContentType(data)

	var img image.Image
	var err error

	switch contentType {
	case "image/png":
		img, err = png.Decode(bytes.NewReader(data))
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("unsupported format: use PNG or JPG")
	}

	if err != nil {
		return nil, fmt.Errorf("could not decode image: %w", err)
	}

	// Resize — fix height at 200px, width scales proportionally
	// Cap width at 800px to prevent extremely wide logos
	// Lanczos3 = highest quality resampling algorithm
	resized := resize.Resize(0, MaxLogoHeight, img, resize.Lanczos3)

	if resized.Bounds().Dx() > MaxLogoWidth {
		resized = resize.Resize(MaxLogoWidth, 0, img, resize.Lanczos3)
	}

	// Always encode as PNG — preserves transparency, best for logos
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil, fmt.Errorf("could not encode logo: %w", err)
	}

	return buf.Bytes(), nil
}
