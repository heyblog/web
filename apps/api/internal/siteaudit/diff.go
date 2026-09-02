package siteaudit

import (
	"fmt"
	"slices"
	"sort"
)

func BuildDiffViews(base, proposed, current Snapshot, final *Snapshot) DiffViews {
	requested := buildSnapshotDiff(base, proposed)
	drift := buildSnapshotDiff(base, current)
	reviewerCorrection := []DiffItem{}
	if final != nil {
		reviewerCorrection = buildSnapshotDiff(proposed, *final)
	}
	return DiffViews{
		Requested:          requested,
		Drift:              drift,
		ReviewerCorrection: reviewerCorrection,
		Conflicts:          conflictingDiffs(requested, drift),
	}
}

func buildSnapshotDiff(before, after Snapshot) []DiffItem {
	items := make([]DiffItem, 0, 12)
	appendScalarDiff(&items, "name", before.Name, after.Name)
	appendScalarDiff(&items, "address", snapshotAddress(before), snapshotAddress(after))
	appendScalarDiff(&items, "summary", before.Summary, after.Summary)
	appendScalarDiff(&items, "access_scope", before.AccessScope, after.AccessScope)
	appendScalarDiff(&items, "visibility", before.Visibility, after.Visibility)
	appendScalarDiff(&items, "visibility_reason", before.VisibilityReason, after.VisibilityReason)
	appendCollectionDiff(&items, "feeds", feedValues(before.Feeds), feedValues(after.Feeds))
	appendCollectionDiff(&items, "resources", resourceValues(before.Resources), resourceValues(after.Resources))
	appendCollectionDiff(&items, "tags", tagValues(before.Tags), tagValues(after.Tags))
	appendCollectionDiff(&items, "components", componentValues(before.Components), componentValues(after.Components))
	appendCollectionDiff(&items, "program_dependencies", componentValues(before.ProgramDependencies), componentValues(after.ProgramDependencies))
	return items
}

func appendScalarDiff(items *[]DiffItem, field, before, after string) {
	if before == after {
		return
	}
	*items = append(*items, DiffItem{Field: field, Before: before, After: after})
}

func appendCollectionDiff(items *[]DiffItem, field string, before, after map[string]string) {
	item := DiffItem{Field: field}
	for key, beforeValue := range before {
		afterValue, exists := after[key]
		if !exists {
			item.Removed = append(item.Removed, beforeValue)
			continue
		}
		if beforeValue != afterValue {
			item.Changed = append(item.Changed, DiffChange{Key: key, Before: beforeValue, After: afterValue})
		}
	}
	for key, afterValue := range after {
		if _, exists := before[key]; !exists {
			item.Added = append(item.Added, afterValue)
		}
	}
	if len(item.Added)+len(item.Removed)+len(item.Changed) == 0 {
		return
	}
	sort.Strings(item.Added)
	sort.Strings(item.Removed)
	slices.SortFunc(item.Changed, func(left, right DiffChange) int {
		if left.Key < right.Key {
			return -1
		}
		if left.Key > right.Key {
			return 1
		}
		return 0
	})
	*items = append(*items, item)
}

func conflictingDiffs(requested, drift []DiffItem) []DiffItem {
	driftByField := make(map[string]DiffItem, len(drift))
	for _, item := range drift {
		driftByField[item.Field] = item
	}
	conflicts := make([]DiffItem, 0)
	for _, requestedItem := range requested {
		driftItem, exists := driftByField[requestedItem.Field]
		if !exists || diffOutcome(requestedItem) == diffOutcome(driftItem) {
			continue
		}
		conflicts = append(conflicts, driftItem)
	}
	return conflicts
}

func diffOutcome(item DiffItem) string {
	return fmt.Sprintf("%s|%v|%v|%v", item.After, item.Added, item.Removed, item.Changed)
}

func snapshotAddress(snapshot Snapshot) string {
	return snapshot.Scheme + "://" + snapshot.NormalizedHost + snapshot.BasePath
}

func feedValues(feeds []FeedSnapshot) map[string]string {
	values := make(map[string]string, len(feeds))
	for _, feed := range feeds {
		key := feed.ID
		if key == "" {
			key = feed.URL
		}
		defaultLabel := ""
		if feed.IsDefault {
			defaultLabel = " · 默认 Feed"
		}
		values[key] = fmt.Sprintf("%s · %s · %s%s", feed.Name, feed.URL, feed.Format, defaultLabel)
	}
	return values
}

func resourceValues(resources []ResourceSnapshot) map[string]string {
	values := make(map[string]string, len(resources))
	for _, resource := range resources {
		values[resource.Kind] = resource.URL
	}
	return values
}

func tagValues(tags []TagSnapshot) map[string]string {
	values := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := tag.ID
		if key == "" {
			key = tag.Role + ":" + tag.SuggestedName
		}
		role := "标签"
		if tag.Role == "PRIMARY" {
			role = "主标签"
		}
		values[key] = role + " · " + firstNonEmpty(tag.Name, tag.SuggestedName, tag.ID)
	}
	return values
}

func componentValues(components []ComponentSnapshot) map[string]string {
	values := make(map[string]string, len(components))
	for _, component := range components {
		key := component.Role + ":" + component.ID
		if component.ID == "" {
			key = component.Role + ":" + component.SuggestedName
		}
		openSource := "未填写"
		if component.IsOpenSource != nil {
			if *component.IsOpenSource {
				openSource = "开源"
			} else {
				openSource = "不开源"
			}
		}
		role := component.Role
		switch component.Role {
		case "SITE_PROGRAM":
			role = "站点程序"
		case "FRAMEWORK":
			role = "框架"
		case "LANGUAGE":
			role = "语言"
		}
		values[key] = fmt.Sprintf("%s · %s\n官网：%s\n仓库：%s\n开源状态：%s", role, firstNonEmpty(component.Name, component.SuggestedName, component.ID), firstNonEmpty(component.HomepageURL, "—"), firstNonEmpty(component.RepositoryURL, "—"), openSource)
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
