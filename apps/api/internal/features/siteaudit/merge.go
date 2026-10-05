package siteaudit

import (
	"reflect"
)

func MergeRequestedSnapshot(base, proposed, current Snapshot) (Snapshot, []DiffItem) {
	merged := current
	conflicts := make([]DiffItem, 0)
	mergeString := func(field string, baseValue, proposedValue, currentValue string, destination *string) {
		if baseValue == proposedValue {
			return
		}
		if currentValue != baseValue && currentValue != proposedValue {
			conflicts = append(conflicts, DiffItem{Field: field, Before: currentValue, After: proposedValue})
			return
		}
		*destination = proposedValue
	}
	mergeString("name", base.Name, proposed.Name, current.Name, &merged.Name)
	mergeString("scheme", base.Scheme, proposed.Scheme, current.Scheme, &merged.Scheme)
	mergeString("normalized_host", base.NormalizedHost, proposed.NormalizedHost, current.NormalizedHost, &merged.NormalizedHost)
	mergeString("base_path", base.BasePath, proposed.BasePath, current.BasePath, &merged.BasePath)
	mergeString("summary", base.Summary, proposed.Summary, current.Summary, &merged.Summary)
	mergeString("access_scope", base.AccessScope, proposed.AccessScope, current.AccessScope, &merged.AccessScope)
	mergeString("visibility", base.Visibility, proposed.Visibility, current.Visibility, &merged.Visibility)
	mergeString("visibility_reason", base.VisibilityReason, proposed.VisibilityReason, current.VisibilityReason, &merged.VisibilityReason)
	mergeSlice("feeds", base.Feeds, proposed.Feeds, current.Feeds, &merged.Feeds, &conflicts)
	mergeSlice("resources", base.Resources, proposed.Resources, current.Resources, &merged.Resources, &conflicts)
	mergeSlice("tags", base.Tags, proposed.Tags, current.Tags, &merged.Tags, &conflicts)
	if !reflect.DeepEqual(base.Tags, proposed.Tags) && reflect.DeepEqual(merged.Tags, proposed.Tags) {
		merged.TagCascadeID = proposed.TagCascadeID
		merged.Classification = proposed.Classification
	}
	mergeSlice("components", base.Components, proposed.Components, current.Components, &merged.Components, &conflicts)
	mergeSlice("program_dependencies", base.ProgramDependencies, proposed.ProgramDependencies, current.ProgramDependencies, &merged.ProgramDependencies, &conflicts)
	return merged, conflicts
}

func mergeSlice[T any](field string, base, proposed, current []T, destination *[]T, conflicts *[]DiffItem) {
	if reflect.DeepEqual(base, proposed) {
		return
	}
	if !reflect.DeepEqual(current, base) && !reflect.DeepEqual(current, proposed) {
		*conflicts = append(*conflicts, DiffItem{Field: field})
		return
	}
	*destination = proposed
}
