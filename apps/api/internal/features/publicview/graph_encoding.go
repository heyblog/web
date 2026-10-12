package publicview

import (
	"bytes"
	"compress/gzip"
	"strconv"
	"strings"
)

func encodeCompressedGraph(data []byte) ([]byte, error) {
	var output bytes.Buffer
	writer, err := gzip.NewWriterLevel(&output, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func acceptsGzip(value string) bool {
	wildcard := false
	for entry := range strings.SplitSeq(value, ",") {
		encoding, parameters, _ := strings.Cut(strings.TrimSpace(entry), ";")
		if encoding != "gzip" && encoding != "*" {
			continue
		}
		quality := 1.0
		if parameters != "" {
			key, raw, ok := strings.Cut(strings.TrimSpace(parameters), "=")
			parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if !ok || key != "q" || err != nil || parsed < 0 || parsed > 1 {
				continue
			}
			quality = parsed
		}
		if encoding == "gzip" {
			return quality > 0
		}
		wildcard = quality > 0
	}
	return wildcard
}
