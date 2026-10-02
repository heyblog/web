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
	seenLevel1 := make(map[string]struct{})
	for _, cascade := range cascades {
		cascadeID, idErr := uuidString(cascade.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level1ID, idErr := uuidString(cascade.Level1ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level2ID, idErr := uuidString(cascade.Level2ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level1 := Option{ID: level1ID, Name: cascade.Level1Name, Level: 1}
		level2 := Option{ID: level2ID, Name: cascade.Level2Name, Level: 2, ParentID: level1ID}
		if _, exists := seenLevel1[level1ID]; !exists {
			options.Tags = append(options.Tags, level1)
			seenLevel1[level1ID] = struct{}{}
		}
		options.Tags = append(options.Tags, level2)
		options.Cascades = append(options.Cascades, CascadeOption{ID: cascadeID, TaxonomyKey: cascade.TaxonomyKey, Level1: level1, Level2: level2})
	}
	for _, tag := range tags {
		id, idErr := uuidString(tag.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
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
