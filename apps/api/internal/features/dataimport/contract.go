package dataimport

import "fmt"

const (
	blogsFormat     = "heyblog.data-import.blogs"
	graphFormat     = "heyblog.data-import.graph"
	taxonomyFormat  = "heyblog.data-import.tag-taxonomy"
	contractVersion = 3
)

type Bundles struct {
	Blogs    BlogBundle
	Graph    GraphBundle
	Taxonomy *TagTaxonomyBundle
}

type InputMetadata struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count"`
}

func DecodeBundles(blogData, graphData []byte) (Bundles, error) {
	var bundles Bundles
	if err := decodeStrictJSON(blogData, &bundles.Blogs); err != nil {
		return Bundles{}, fmt.Errorf("decode blogs bundle: %w", err)
	}
	if err := decodeStrictJSON(graphData, &bundles.Graph); err != nil {
		return Bundles{}, fmt.Errorf("decode graph bundle: %w", err)
	}
	if err := validateCleanedJSONShape(blogData, graphData); err != nil {
		return Bundles{}, err
	}
	if err := validateBundles(bundles); err != nil {
		return Bundles{}, err
	}
	return bundles, nil
}

func DecodeTagTaxonomyBundle(data []byte) (TagTaxonomyBundle, error) {
	var bundle TagTaxonomyBundle
	if err := decodeStrictJSON(data, &bundle); err != nil {
		return TagTaxonomyBundle{}, fmt.Errorf("decode tag taxonomy bundle: %w", err)
	}
	if err := validateTagTaxonomyBundle(bundle); err != nil {
		return TagTaxonomyBundle{}, err
	}
	return bundle, nil
}
