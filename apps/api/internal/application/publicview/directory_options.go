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
	PrimaryTags   []DirectoryOption `json:"primaryTags"`
	SecondaryTags []DirectoryOption `json:"secondaryTags"`
	Warnings      []DirectoryOption `json:"warnings"`
	Technologies  []DirectoryOption `json:"technologies"`
}

func (service *Service) DirectoryOptions(ctx context.Context) (DirectoryOptions, error) {
	tags, err := service.queries.ListDirectoryTagOptions(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory tag options")
	}
	technologies, err := service.queries.ListDirectoryTechnologyOptions(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory technology options")
	}
	options := DirectoryOptions{
		PrimaryTags: []DirectoryOption{}, SecondaryTags: []DirectoryOption{},
		Warnings: []DirectoryOption{}, Technologies: []DirectoryOption{},
	}
	for _, tag := range tags {
		option := DirectoryOption{
			Value: tag.Slug, Label: tag.Name,
			NormalCount: tag.NormalCount, AbnormalCount: tag.AbnormalCount,
		}
		switch tag.Role {
		case "PRIMARY":
			options.PrimaryTags = append(options.PrimaryTags, option)
		case "SECONDARY":
			options.SecondaryTags = append(options.SecondaryTags, option)
		case "WARNING":
			options.Warnings = append(options.Warnings, option)
		default:
			return DirectoryOptions{}, internalError(
				fmt.Errorf("unsupported directory tag role %q", tag.Role),
				"map directory tag option",
			)
		}
	}
	for _, technology := range technologies {
		options.Technologies = append(options.Technologies, DirectoryOption{
			Value: technology.NormalizedName, Label: technology.Name,
			NormalCount: technology.NormalCount, AbnormalCount: technology.AbnormalCount,
		})
	}
	return options, nil
}
