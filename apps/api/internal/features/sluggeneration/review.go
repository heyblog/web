package sluggeneration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"heyblog-api/internal/features/auth"
	"heyblog-api/internal/features/siteaudit"
	"heyblog-api/internal/infrastructure/tokenhub"
	"heyblog-api/internal/platform/apperror"
)

type tagSlugLookup interface {
	ExistingSlug(context.Context, string) (string, error)
}

// ReviewedTags performs at most two provider calls, outside audit transactions.
func (service *Service) ReviewedTags(ctx context.Context, user auth.User, ip string, tags []siteaudit.TagSnapshot) ([]siteaudit.TagSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	result := slices.Clone(tags)
	inputs := make([]tokenhub.BatchInput, 0, 20)
	seen := map[string]string{}
	reserved := map[string]bool{}
	for _, tag := range result {
		if tag.Slug != "" {
			reserved[tag.Slug] = true
		}
	}
	for i, tag := range result {
		if tag.ID != "" || tag.Slug != "" {
			continue
		}
		name := strings.TrimSpace(tag.SuggestedName)
		normalized := strings.ToLower(name)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = ""
		if lookup, ok := service.store.(tagSlugLookup); ok {
			slug, err := lookup.ExistingSlug(ctx, normalized)
			if err != nil {
				return nil, unavailable()
			}
			if slug != "" {
				seen[normalized] = slug
				continue
			}
		}
		if user.Role != auth.RoleSysAdmin && (user.Role != auth.RoleAdmin || !slices.Contains(user.Permissions, auth.PermissionTaxonomyManage)) {
			return nil, apperror.New(apperror.KindForbidden, "taxonomy_permission_required", "taxonomy management permission is required for new tags")
		}
		inputs = append(inputs, tokenhub.BatchInput{ID: fmt.Sprintf("review-%d", i), Name: name, Description: strings.TrimSpace(tag.Description)})
	}
	if len(inputs) > 20 {
		return nil, apperror.New(apperror.KindValidation, "invalid_tag", "at most twenty new tags may be reviewed")
	}
	if len(inputs) > 0 {
		if err := service.guard.Allow(ctx, Identity{UserID: user.ID, IP: ip}); err != nil {
			return nil, err
		}
		settings, err := service.Settings(ctx)
		if err != nil {
			return nil, err
		}
		for start := 0; start < len(inputs); start += 10 {
			chunk := inputs[start:min(start+10, len(inputs))]
			candidates, err := service.reviewCandidates(ctx, Identity{UserID: user.ID, IP: ip}, settings.ModelID, chunk)
			if err != nil {
				return nil, err
			}
			for _, input := range chunk {
				base := candidates[input.ID]
				slug, err := service.unique(ctx, base, "")
				if err != nil {
					return nil, err
				}
				for suffix := 2; reserved[slug]; suffix++ {
					if suffix > 10000 {
						return nil, jobFailure("slug_collision")
					}
					tail := fmt.Sprintf("-%d", suffix)
					candidate := strings.TrimRight(base[:min(len(base), 128-len(tail))], "-") + tail
					slug, err = service.unique(ctx, candidate, "")
					if err != nil {
						return nil, err
					}
				}
				reserved[slug] = true
				seen[strings.ToLower(input.Name)] = slug
			}
		}
	}
	for i, tag := range result {
		if tag.ID == "" && tag.Slug == "" {
			result[i].Slug = seen[strings.ToLower(strings.TrimSpace(tag.SuggestedName))]
		}
	}
	return result, nil
}
