package dataimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

const (
	BlogsFileLimit int64 = 64 << 20
	GraphFileLimit int64 = 16 << 20
	TotalBodyLimit int64 = 80 << 20
)

type uploadedBundles struct {
	Bundles     Bundles
	BlogsSHA256 string
	GraphSHA256 string
}

var (
	errMalformedUpload = errors.New("malformed data import upload")
	errUploadTooLarge  = errors.New("data import upload is too large")
	errInvalidContract = errors.New("invalid cleaned data contract")
)

func decodeUpload(request *http.Request) (uploadedBundles, error) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return uploadedBundles{}, errMalformedUpload
	}
	reader, err := request.MultipartReader()
	if err != nil {
		return uploadedBundles{}, fmt.Errorf("%w: multipart reader", errMalformedUpload)
	}
	files := make(map[string][]byte, 2)
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(nextErr, &tooLarge) {
				return uploadedBundles{}, errUploadTooLarge
			}
			return uploadedBundles{}, fmt.Errorf("%w: read multipart part", errMalformedUpload)
		}
		name := part.FormName()
		limit, known := map[string]int64{"blogs": BlogsFileLimit, "graph": GraphFileLimit}[name]
		if !known || part.FileName() == "" {
			_ = part.Close()
			return uploadedBundles{}, fmt.Errorf("%w: unexpected multipart field", errMalformedUpload)
		}
		if _, exists := files[name]; exists {
			_ = part.Close()
			return uploadedBundles{}, fmt.Errorf("%w: duplicate multipart field", errMalformedUpload)
		}
		contents, readErr := io.ReadAll(io.LimitReader(part, limit+1))
		closeErr := part.Close()
		var readTooLarge *http.MaxBytesError
		var closeTooLarge *http.MaxBytesError
		if errors.As(readErr, &readTooLarge) || errors.As(closeErr, &closeTooLarge) {
			return uploadedBundles{}, errUploadTooLarge
		}
		if readErr != nil || closeErr != nil {
			return uploadedBundles{}, fmt.Errorf("%w: read multipart file", errMalformedUpload)
		}
		if int64(len(contents)) > limit {
			return uploadedBundles{}, errUploadTooLarge
		}
		files[name] = contents
	}
	blogs, hasBlogs := files["blogs"]
	graph, hasGraph := files["graph"]
	if !hasBlogs || !hasGraph {
		return uploadedBundles{}, fmt.Errorf("%w: blogs and graph files are required", errMalformedUpload)
	}
	if !json.Valid(blogs) || !json.Valid(graph) {
		return uploadedBundles{}, fmt.Errorf("%w: invalid JSON", errMalformedUpload)
	}
	bundles, err := DecodeBundles(blogs, graph)
	if err != nil {
		return uploadedBundles{}, fmt.Errorf("%w: %w", errInvalidContract, err)
	}
	return uploadedBundles{
		Bundles: bundles, BlogsSHA256: hashHex(blogs), GraphSHA256: hashHex(graph),
	}, nil
}

func hashHex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
