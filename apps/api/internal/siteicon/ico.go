package siteicon

import (
	"bytes"
	"encoding/binary"
	"image/png"

	_ "github.com/sergeymakinen/go-ico"
)

// Preflight every directory entry before passing a single bounded entry to the
// decoder. Its BMP palette reader assumes at most 256 colors, and its generic
// image reader otherwise buffers lengths taken directly from the ICO header.
func largestICO(content []byte) ([]byte, error) {
	if len(content) < 6 {
		return nil, ErrInvalid
	}
	count := int(binary.LittleEndian.Uint16(content[4:6]))
	if count == 0 || count > 256 || 6+16*count > len(content) {
		return nil, ErrInvalid
	}
	bestArea, bestEntry := 0, 0
	var bestData []byte
	for index := range count {
		entry := 6 + 16*index
		size := uint64(binary.LittleEndian.Uint32(content[entry+8:]))
		offset := uint64(binary.LittleEndian.Uint32(content[entry+12:]))
		if offset < uint64(6+16*count) || size == 0 || offset+size > uint64(len(content)) {
			return nil, ErrInvalid
		}
		data := content[offset : offset+size]
		width, height, err := icoDimensions(data)
		if err != nil || !boundedDimensions(width, height) {
			return nil, ErrInvalid
		}
		if width*height > bestArea {
			bestArea, bestEntry, bestData = width*height, entry, data
		}
	}
	// Selecting by area ourselves avoids the library's preference for a smaller
	// entry solely because it has a higher bit depth.
	result := make([]byte, 22+len(bestData))
	copy(result, content[:6])
	binary.LittleEndian.PutUint16(result[4:], 1)
	copy(result[6:22], content[bestEntry:bestEntry+16])
	binary.LittleEndian.PutUint32(result[18:], 22)
	copy(result[22:], bestData)
	return result, nil
}

func icoDimensions(data []byte) (int, int, error) {
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		config, err := png.DecodeConfig(bytes.NewReader(data))
		return config.Width, config.Height, err
	}
	if len(data) < 40 {
		return 0, 0, ErrInvalid
	}
	headerSize := binary.LittleEndian.Uint32(data)
	if (headerSize != 40 && headerSize != 108 && headerSize != 124) || uint64(headerSize) > uint64(len(data)) {
		return 0, 0, ErrInvalid
	}
	width := binary.LittleEndian.Uint32(data[4:])
	if width == 0 || width > 2048 {
		return 0, 0, ErrInvalid
	}
	height := int64(binary.LittleEndian.Uint32(data[8:]))
	if height >= 1<<31 {
		height = 1<<32 - height
	}
	if height%2 != 0 || height > 4096 {
		return 0, 0, ErrInvalid
	}
	colors := binary.LittleEndian.Uint32(data[32:])
	if colors > 256 {
		return 0, 0, ErrInvalid
	}
	return int(width), int(height / 2), nil
}
