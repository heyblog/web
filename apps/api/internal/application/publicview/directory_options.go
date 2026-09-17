package publicview

import (
	"context"
	"fmt"
)

type DirectoryOption struct {
	Value         string `json:"value"`
	Label         string `json:"label"`
	NormalCount   int64  `json:"normalCount"`
	AbnormalCount int64  `json:"abnormalCount"`
}

type DirectoryOptions struct {
	Classifications []DirectoryClassificationOption `json:"classifications"`
	TertiaryTags    []DirectoryOption               `json:"tertiaryTags"`
	Warnings        []DirectoryOption               `json:"warnings"`
	Technologies    []DirectoryOption               `json:"technologies"`
}

type DirectoryClassificationOption struct {
	DirectoryOption
	Children []DirectoryOption `json:"children"`
}

func (service *Service) DirectoryOptions(ctx context.Context) (DirectoryOptions, error) {
	tags, err := service.queries.ListDirectoryTagOptions(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory tag options")
	}
	cascades, err := service.queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory classifications")
	}
	technologies, err := service.queries.ListDirectoryTechnologyOptions(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory technology options")
	}
	options := DirectoryOptions{
		Classifications: []DirectoryClassificationOption{}, TertiaryTags: []DirectoryOption{},
		Warnings: []DirectoryOption{}, Technologies: []DirectoryOption{},
	}
	counts := make(map[string]DirectoryOption, len(tags))
	for _, tag := range tags {
		option := DirectoryOption{
			Value: tag.Slug, Label: tag.Name,
			NormalCount: tag.NormalCount, AbnormalCount: tag.AbnormalCount,
		}
		switch tag.Role {
		case "PRIMARY":
			counts["PRIMARY:"+tag.Slug] = option
		case "SECONDARY":
			counts["SECONDARY:"+tag.Slug] = option
		case "TERTIARY":
			options.TertiaryTags = append(options.TertiaryTags, option)
		case "WARNING":
			options.Warnings = append(options.Warnings, option)
		default:
			return DirectoryOptions{}, internalError(
				fmt.Errorf("unsupported directory tag role %q", tag.Role),
				"map directory tag option",
			)
		}
	}
	classificationIndex := make(map[string]int)
	for _, cascade := range cascades {
		level1 := counts["PRIMARY:"+cascade.Level1Slug]
		level1.Value, level1.Label = cascade.Level1Slug, cascade.Level1Name
		level2 := counts["SECONDARY:"+cascade.Level2Slug]
		level2.Value, level2.Label = cascade.Level2Slug, cascade.Level2Name
		index, exists := classificationIndex[level1.Value]
		if !exists {
			index = len(options.Classifications)
			classificationIndex[level1.Value] = index
			options.Classifications = append(options.Classifications, DirectoryClassificationOption{
				DirectoryOption: level1, Children: []DirectoryOption{},
			})
		}
		options.Classifications[index].Children = append(options.Classifications[index].Children, level2)
	}
	for _, technology := range technologies {
		options.Technologies = append(options.Technologies, DirectoryOption{
			Value: technology.NormalizedName, Label: technology.Name,
			NormalCount: technology.NormalCount, AbnormalCount: technology.AbnormalCount,
		})
	}
	return options, nil
}
