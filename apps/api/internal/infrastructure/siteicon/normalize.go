// Package siteicon normalizes bounded, cached raster icons for presentation.
package siteicon

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var ErrInvalid = errors.New("invalid cached icon")

// Normalize returns a PNG no larger than 128 pixels on either axis.
func Normalize(content []byte) ([]byte, error) {
	if len(content) == 0 || len(content) > 1<<20 {
		return nil, ErrInvalid
	}
	input := content
	if bytes.HasPrefix(content, []byte{0, 0, 1, 0}) {
		var err error
		input, err = largestICO(content)
		if err != nil {
			return nil, err
		}
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("read icon dimensions: %w", err)
	}
	switch format {
	case "png", "jpeg", "gif", "webp", "ico":
	default:
		return nil, ErrInvalid
	}
	if !boundedDimensions(config.Width, config.Height) {
		return nil, ErrInvalid
	}
	decoded, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("decode icon: %w", err)
	}
	// BMP-backed ICO decoders preserve palette indices without checking that
	// each index exists; image.At would panic when scaling a malformed bitmap.
	if paletted, ok := decoded.(*image.Paletted); ok {
		for _, index := range paletted.Pix {
			if int(index) >= len(paletted.Palette) {
				return nil, ErrInvalid
			}
		}
	}
	width, height := config.Width, config.Height
	if longest := max(width, height); longest > 128 {
		width, height = max(1, width*128/longest), max(1, height*128/longest)
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), decoded, decoded.Bounds(), draw.Src, nil)
	var output bytes.Buffer
	if err := png.Encode(&output, scaled); err != nil {
		return nil, fmt.Errorf("encode icon: %w", err)
	}
	return output.Bytes(), nil
}

func boundedDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= 2048 && height <= 2048 && width*height <= 4_000_000
}
