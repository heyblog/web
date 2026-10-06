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
	tags, err := repository.queries.ListEnabledTagLabels(ctx)
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
	labelsByTag := map[string][]Option{}
	for _, label := range tags {
		id, idErr := uuidString(label.TagID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		labelID, idErr := uuidString(label.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		labelsByTag[id] = append(labelsByTag[id], Option{ID: id, LabelID: labelID, Name: label.Name, Slug: label.Slug, Level: 3})
	}
	for id, labels := range labelsByTag {
		names := make([]string, 0, len(labels))
		for _, label := range labels {
			names = append(names, label.Name)
		}
		for i := range labels {
			labels[i].Synonyms = names
		}
		labelsByTag[id] = labels
	}
	seen := map[string]bool{}
	for _, cascade := range cascades {
		cascadeID, idErr := uuidString(cascade.ID)
		if idErr != nil {
			return SubmissionOptions{}, idErr
		}
		level1ID, _ := uuidString(cascade.Level1ID)
		level2ID, _ := uuidString(cascade.Level2ID)
		var level1, level2 Option
		for _, label := range labelsByTag[level1ID] {
			label.Level = 1
			if label.Name == cascade.Level1Name {
				level1 = label
			}
			key := "1:" + label.LabelID
			if !seen[key] {
				options.Tags = append(options.Tags, label)
				seen[key] = true
			}
		}
		for _, label := range labelsByTag[level2ID] {
			label.Level, label.ParentID = 2, level1ID
			if label.Name == cascade.Level2Name {
				level2 = label
			}
			key := "2:" + level1ID + ":" + label.LabelID
			if !seen[key] {
				options.Tags = append(options.Tags, label)
				seen[key] = true
			}
		}
		options.Cascades = append(options.Cascades, CascadeOption{ID: cascadeID, TaxonomyKey: cascade.TaxonomyKey, Level1: level1, Level2: level2})
	}
	for _, label := range tags {
		id, _ := uuidString(label.TagID)
		for _, option := range labelsByTag[id] {
			key := "3:" + option.LabelID
			if !seen[key] {
				options.Tags = append(options.Tags, option)
				seen[key] = true
			}
		}
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
