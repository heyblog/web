package siteicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/sergeymakinen/go-ico"
)

func TestNormalizeRasterFormats(t *testing.T) {
	t.Parallel()
	for name, encode := range map[string]func(io.Writer, image.Image) error{
		"png":  png.Encode,
		"jpeg": func(w io.Writer, m image.Image) error { return jpeg.Encode(w, m, nil) },
		"gif":  func(w io.Writer, m image.Image) error { return gif.Encode(w, m, nil) },
		"ico":  ico.Encode,
	} {
		t.Run(name, func(t *testing.T) {
			// Given a wide cached image.
			var input bytes.Buffer
			if err := encode(&input, image.NewNRGBA(image.Rect(0, 0, 256, 128))); err != nil {
				t.Fatal(err)
			}
			// When normalizing it for a card.
			output, err := Normalize(input.Bytes())
			// Then it is a proportionally scaled PNG.
			if err != nil {
				t.Fatal(err)
			}
			config, err := png.DecodeConfig(bytes.NewReader(output))
			if err != nil || config.Width != 128 || config.Height != 64 {
				t.Fatalf("config=%+v error=%v", config, err)
			}
		})
	}
}

func TestNormalizePreservesAlphaWithoutUpscaling(t *testing.T) {
	t.Parallel()
	// Given a small translucent icon.
	source := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	source.SetNRGBA(1, 1, color.NRGBA{R: 200, A: 128})
	var input bytes.Buffer
	if err := png.Encode(&input, source); err != nil {
		t.Fatal(err)
	}
	// When normalizing.
	output, err := Normalize(input.Bytes())
	// Then dimensions and alpha survive.
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := decoded.At(1, 1).RGBA()
	if decoded.Bounds() != source.Bounds() || alpha != 128*257 {
		t.Fatalf("bounds=%v alpha=%d", decoded.Bounds(), alpha)
	}
}

func TestNormalizeRejectsUnboundedOrCorruptInput(t *testing.T) {
	t.Parallel()
	var oversized bytes.Buffer
	if err := png.Encode(&oversized, image.NewGray(image.Rect(0, 0, 2049, 1))); err != nil {
		t.Fatal(err)
	}
	var icoSize [22]byte
	copy(icoSize[:], []byte{0, 0, 1, 0, 1, 0})
	binary.LittleEndian.PutUint32(icoSize[14:], 0xffffffff)
	binary.LittleEndian.PutUint32(icoSize[18:], 22)
	for name, input := range map[string][]byte{
		"empty": nil, "corrupt": []byte("not an image"), "bytes": make([]byte, 1<<20+1),
		"dimensions": oversized.Bytes(), "ico allocation": icoSize[:],
	} {
		t.Run(name, func(t *testing.T) {
			// Given malformed or oversized bytes; when normalizing; then reject safely.
			if _, err := Normalize(input); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
