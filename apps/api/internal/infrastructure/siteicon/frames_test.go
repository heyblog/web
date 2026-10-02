package siteicon

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"

	"github.com/sergeymakinen/go-ico"
)

func TestNormalizeWebP(t *testing.T) {
	t.Parallel()
	// Given a one-pixel VP8 WebP fixture.
	input, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	// When normalizing.
	output, err := Normalize(input)
	// Then it becomes a one-pixel PNG.
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil || config.Width != 1 || config.Height != 1 {
		t.Fatalf("config=%+v err=%v", config, err)
	}
}

func TestNormalizeUsesFirstGIFFrame(t *testing.T) {
	t.Parallel()
	// Given an animation whose frames have different colors.
	first := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.RGBA{R: 255, A: 255}})
	second := image.NewPaletted(first.Bounds(), color.Palette{color.RGBA{B: 255, A: 255}})
	var input bytes.Buffer
	if err := gif.EncodeAll(&input, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	// When normalizing.
	output, err := Normalize(input.Bytes())
	// Then only the first frame is shown.
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	r, _, b, _ := decoded.At(0, 0).RGBA()
	if r != 65535 || b != 0 {
		t.Fatalf("red=%d blue=%d", r, b)
	}
}

func TestNormalizeSelectsLargestICORegardlessOfBitDepth(t *testing.T) {
	t.Parallel()
	// Given a large low-bit-depth image followed by a small true-color image.
	large := image.NewPaletted(image.Rect(0, 0, 64, 64), color.Palette{color.White, color.Black})
	small := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	var input bytes.Buffer
	if err := ico.EncodeAll(&input, []image.Image{large, small}); err != nil {
		t.Fatal(err)
	}
	// When normalizing.
	output, err := Normalize(input.Bytes())
	// Then the largest image wins.
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(output))
	if err != nil || config.Width != 64 || config.Height != 64 {
		t.Fatalf("config=%+v err=%v", config, err)
	}
}

func TestNormalizeRejectsICOUnboundedPalette(t *testing.T) {
	t.Parallel()
	// Given a valid ICO with a forged BMP palette count.
	var input bytes.Buffer
	if err := ico.Encode(&input, image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.White, color.Black})); err != nil {
		t.Fatal(err)
	}
	data := input.Bytes()
	offset := binary.LittleEndian.Uint32(data[18:])
	binary.LittleEndian.PutUint32(data[offset+32:], 0xffffffff)
	// When normalizing; then reject before the BMP decoder reads its fixed palette buffer.
	if _, err := Normalize(data); err == nil {
		t.Fatal("expected invalid palette rejection")
	}
}

func TestNormalizeRejectsICOInvalidPaletteIndex(t *testing.T) {
	t.Parallel()
	// Given a one-pixel 8-bit BMP icon whose pixel is outside its one-color palette.
	data := make([]byte, 74)
	copy(data, []byte{0, 0, 1, 0, 1, 0, 1, 1})
	binary.LittleEndian.PutUint32(data[14:], 52)
	binary.LittleEndian.PutUint32(data[18:], 22)
	binary.LittleEndian.PutUint32(data[22:], 40)
	binary.LittleEndian.PutUint32(data[26:], 1)
	binary.LittleEndian.PutUint32(data[30:], 2)
	binary.LittleEndian.PutUint16(data[34:], 1)
	binary.LittleEndian.PutUint16(data[36:], 8)
	binary.LittleEndian.PutUint32(data[54:], 1)
	data[66] = 255
	// When normalizing; then reject before image scaling calls At on that pixel.
	if _, err := Normalize(data); err == nil {
		t.Fatal("expected invalid palette index rejection")
	}
}
