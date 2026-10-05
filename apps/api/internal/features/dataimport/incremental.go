package dataimport

import (
	"context"
	"fmt"
)

type ImportMode string

const (
	ImportInitial        ImportMode = "initial"
	ImportIncremental    ImportMode = "incremental"
	friendGraphSourceKey            = "FRIEND_GRAPH_JSONL"
)

type incrementalStore interface {
	ImportIncremental(context.Context, Plan, func() (string, error)) (Counts, error)
}

func friendGraphInputs(inputs []InputMetadata) bool {
	return len(inputs) == 2 && inputs[0].Kind == "nodes" && inputs[1].Kind == "edges"
}

func validateImportMode(bundles Bundles) error {
	switch bundles.Mode {
	case "", ImportInitial:
		return nil
	case ImportIncremental:
		if bundles.Taxonomy != nil || !friendGraphInputs(bundles.Blogs.Inputs) {
			return fmt.Errorf("incremental mode requires nodes and edges bundles")
		}
		for index, blog := range bundles.Blogs.Blogs {
			if blog.Visibility != "VISIBLE" || blog.Sitemap != nil || blog.LinkPage != nil ||
				blog.MainTag != nil || len(blog.SubTags) != 0 || blog.Architecture != nil {
				return fmt.Errorf("blogs[%d] contains unsupported incremental profile fields", index)
			}
			if len(blog.Origins) != 1 || blog.Origins[0].SourceKey != friendGraphSourceKey {
				return fmt.Errorf("blogs[%d] requires one friend graph origin", index)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported import mode %q", bundles.Mode)
	}
}
