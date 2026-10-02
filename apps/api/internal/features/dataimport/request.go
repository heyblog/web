package dataimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	BlogsFileLimit    int64 = 64 << 20
	GraphFileLimit    int64 = 16 << 20
	TaxonomyFileLimit int64 = 64 << 20
	TotalBodyLimit    int64 = 80 << 20
)

type uploadedBundles struct {
	Bundles        Bundles
	BlogsSHA256    string
	GraphSHA256    string
	TaxonomySHA256 string
}

var (
	errMalformedUpload = errors.New("malformed data import upload")
	errUploadTooLarge  = errors.New("data import upload is too large")
	errInvalidContract = errors.New("invalid cleaned data contract")
)

func decodeUploadedFiles(files map[string][]byte) (uploadedBundles, error) {
	blogs, hasBlogs := files["blogs"]
	graph, hasGraph := files["graph"]
	taxonomy, hasTaxonomy := files["taxonomy"]
	if hasTaxonomy {
		if hasBlogs || hasGraph || !json.Valid(taxonomy) {
			return uploadedBundles{}, fmt.Errorf("%w: taxonomy must be uploaded alone as valid JSON", errMalformedUpload)
		}
		bundle, decodeErr := DecodeTagTaxonomyBundle(taxonomy)
		if decodeErr != nil {
			return uploadedBundles{}, fmt.Errorf("%w: %w", errInvalidContract, decodeErr)
		}
		return uploadedBundles{Bundles: Bundles{Taxonomy: &bundle}, TaxonomySHA256: hashHex(taxonomy)}, nil
	}
	if !hasBlogs || !hasGraph {
		return uploadedBundles{}, fmt.Errorf("%w: blogs and graph files or one taxonomy file are required", errMalformedUpload)
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
