package publicview

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type DirectoryOption struct {
	ID            string `json:"id,omitempty"`
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
	tags, err := service.options.ListDirectoryTagOptions(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory tag options")
	}
	cascades, err := service.options.ListPublicSiteTagCascades(ctx)
	if err != nil {
		return DirectoryOptions{}, internalError(err, "list directory classifications")
	}
	technologies, err := service.options.ListDirectoryTechnologyOptions(ctx)
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
			ID: uuidText(tag.ID), Value: tag.Name, Label: tag.Name,
			NormalCount: tag.NormalCount, AbnormalCount: tag.AbnormalCount,
		}
		switch tag.Role {
		case "PRIMARY":
			counts["PRIMARY:"+tag.Name] = option
		case "SECONDARY":
			counts["SECONDARY:"+tag.Name] = option
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
		level1 := counts["PRIMARY:"+cascade.Level1Name]
		level1.Value, level1.Label, level1.ID = cascade.Level1Name, cascade.Level1Name, uuidText(cascade.Level1ID)
		level2 := counts["SECONDARY:"+cascade.Level2Name]
		level2.Value, level2.Label, level2.ID = cascade.Level2Name, cascade.Level2Name, uuidText(cascade.Level2ID)
		key := level1.ID
		index, exists := classificationIndex[key]
		if !exists {
			index = len(options.Classifications)
			classificationIndex[key] = index
			options.Classifications = append(options.Classifications, DirectoryClassificationOption{
				DirectoryOption: level1, Children: []DirectoryOption{},
			})
		}
		duplicate := false
		for _, child := range options.Classifications[index].Children {
			if child.ID == level2.ID {
				duplicate = true
				break
			}
		}
		if !duplicate {
			options.Classifications[index].Children = append(options.Classifications[index].Children, level2)
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

func uuidText(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:])
}
