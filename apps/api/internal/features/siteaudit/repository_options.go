package siteaudit

import (
	"context"
	"fmt"
)

const privateProgramNormalizedName = "其他"

func (repository *Repository) Options(ctx context.Context) (SubmissionOptions, error) {
	cascades, err := repository.queries.ListEnabledSiteTagCascades(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission tag cascades: %w", err)
	}
	tags, err := repository.queries.ListEnabledTags(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission tag options: %w", err)
	}
	components, err := repository.queries.ListEnabledSoftwareComponents(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission component options: %w", err)
	}
	dependencies, err := repository.queries.ListEnabledSoftwareComponentDependencies(ctx)
	if err != nil {
		return SubmissionOptions{}, fmt.Errorf("list submission program dependencies: %w", err)
	}
	options := SubmissionOptions{
		Tags: make([]Option, 0, len(tags)+len(cascades)*2), Cascades: make([]CascadeOption, 0, len(cascades)), Components: make([]ComponentOption, 0, len(components)),
		ProgramDependencies: make([]ProgramDependencyOption, 0, len(dependencies)),
	}
	seen := map[string]bool{}
	for _, cascade := range cascades {
		cascadeID, err := uuidString(cascade.ID)
		if err != nil {
			return SubmissionOptions{}, err
		}
		primaryID, _ := uuidString(cascade.Level1ID)
		secondaryID, _ := uuidString(cascade.Level2ID)
		primary := Option{ID: primaryID, Name: cascade.Level1Name, Level: 1}
		secondary := Option{ID: secondaryID, Name: cascade.Level2Name, Level: 2, ParentID: primaryID}
		for _, option := range []Option{primary, secondary} {
			key := fmt.Sprint(option.Level) + ":" + option.ParentID + ":" + option.ID
			if !seen[key] {
				options.Tags = append(options.Tags, option)
				seen[key] = true
			}
		}
		options.Cascades = append(options.Cascades, CascadeOption{ID: cascadeID, TaxonomyKey: cascade.TaxonomyKey, Level1: primary, Level2: secondary})
	}
	for _, tag := range tags {
		id, err := uuidString(tag.ID)
		if err != nil {
			return SubmissionOptions{}, err
		}
		options.Tags = append(options.Tags, Option{ID: id, Name: tag.Name, Level: 3})
	}

	for _, component := range components {
		id, idErr := uuidString(component.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		options.Components = append(options.Components, ComponentOption{ID: id, Name: component.Name, HomepageURL: stringValue(component.HomepageUrl), RepositoryURL: stringValue(component.RepositoryUrl), IsOpenSource: component.IsOpenSource})
		if component.NormalizedName == privateProgramNormalizedName {
			options.PrivateProgramID = id
		}
	}
	for _, dependency := range dependencies {
		programID, idErr := uuidString(dependency.ComponentID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		componentID, idErr := uuidString(dependency.DependencyComponentID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		options.ProgramDependencies = append(options.ProgramDependencies, ProgramDependencyOption{ProgramID: programID, ComponentID: componentID, Role: dependency.Role})
	}
	return options, nil
}
