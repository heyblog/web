package siteaudit

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/infrastructure/database/gen"
)

type annotationTagChange struct {
	before *dbgen.DirectorySiteTag
	after  *dbgen.AssignSiteTagParams
}

func (write annotationWrite) prepareTags(ctx context.Context, before AnnotationDetail) (pgtype.UUID, []annotationTagChange, error) {
	cascadeID := write.row.TagCascadeID
	if write.patch.TagCascadeID != nil && *write.patch.TagCascadeID != before.TagCascadeID {
		id, err := parseUUID(*write.patch.TagCascadeID)
		if err != nil {
			return pgtype.UUID{}, nil, invalidAnnotation("the selected site classification is invalid")
		}
		cascade, err := write.q.GetCanonicalCascade(ctx, id)
		if isNotFound(err) || err == nil && (!cascade.IsEnabled || cascade.Scope != "SITE") {
			return pgtype.UUID{}, nil, invalidAnnotation("the selected site classification is unavailable")
		}
		if err != nil {
			return pgtype.UUID{}, nil, err
		}
		cascadeID = cascade.ID
	}
	if write.patch.Tags == nil && cascadeID == write.row.TagCascadeID {
		return cascadeID, nil, nil
	}
	cascade, err := write.q.GetReadableSiteTagCascade(ctx, cascadeID)
	if err != nil {
		return pgtype.UUID{}, nil, err
	}
	old, err := write.q.ListSiteTags(ctx, write.row.ID)
	if err != nil {
		return pgtype.UUID{}, nil, err
	}
	current := make(map[pgtype.UUID]dbgen.DirectorySiteTag, len(old))
	for _, tag := range old {
		current[tag.TagID] = dbgen.DirectorySiteTag{TagID: tag.TagID, Role: tag.Role, AssignmentSource: tag.AssignmentSource, Position: tag.Position, Note: tag.Note}
	}
	next := write.patch.Tags
	if next == nil {
		for _, tag := range before.Tags {
			if tag.Level == 3 {
				next = append(next, tag.AnnotationSelection)
			}
		}
	}
	seen := make(map[pgtype.UUID]struct{}, len(next))
	var changes []annotationTagChange
	var position int16
	for _, tag := range next {
		id, err := parseUUID(tag.ID)
		if err != nil || (tag.Role != "TERTIARY" && tag.Role != "WARNING") {
			return pgtype.UUID{}, nil, invalidAnnotation("a selected tag is invalid")
		}
		if _, duplicate := seen[id]; duplicate {
			return pgtype.UUID{}, nil, invalidAnnotation("selected tags must be unique")
		}
		seen[id] = struct{}{}
		if tag.Role == "TERTIARY" && (id == cascade.Level1ID || id == cascade.Level2ID) {
			return pgtype.UUID{}, nil, invalidAnnotation("a tertiary tag cannot repeat the site classification")
		}
		selected := dbgen.AssignSiteTagParams{SiteID: write.row.ID, TagID: id, Role: tag.Role, AssignmentSource: "MANUAL"}
		if tag.Role == "TERTIARY" {
			position++
			if position > 20 {
				return pgtype.UUID{}, nil, invalidAnnotation("a site can have at most twenty tertiary tags")
			}
			value := position
			selected.Position = &value
		}
		prior, assigned := current[id]
		if assigned && prior.Role == tag.Role {
			if sameTagPosition(prior.Position, selected.Position) {
				continue
			}
			selected.Note, selected.AssignmentSource = prior.Note, prior.AssignmentSource
		} else {
			canonical, err := write.q.GetCanonicalTag(ctx, id)
			if isNotFound(err) || err == nil && (!canonical.IsEnabled || canonical.ID != id) {
				return pgtype.UUID{}, nil, invalidAnnotation("a selected tag is unavailable")
			}
			if err != nil {
				return pgtype.UUID{}, nil, err
			}
		}
		change := annotationTagChange{after: &selected}
		if assigned {
			change.before = &prior
		}
		changes = append(changes, change)
	}
	for id, tag := range current {
		if _, exists := seen[id]; !exists {
			changes = append(changes, annotationTagChange{before: &tag})
		}
	}
	return cascadeID, changes, nil
}

func sameTagPosition(a, b *int16) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (write annotationWrite) removeChangedTags(ctx context.Context, changes []annotationTagChange) error {
	for _, change := range changes {
		if change.before != nil {
			if err := write.q.UnassignSiteTag(ctx, dbgen.UnassignSiteTagParams{SiteID: write.row.ID, TagID: change.before.TagID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (write annotationWrite) addChangedTags(ctx context.Context, changes []annotationTagChange) error {
	for _, change := range changes {
		if change.after != nil {
			if _, err := write.q.AssignSiteTag(ctx, *change.after); err != nil {
				return err
			}
		}
	}
	return nil
}

func (write annotationWrite) saveComponents(ctx context.Context, before []AnnotationComponent) error {
	roles := []string{"SITE_PROGRAM", "FRAMEWORK", "LANGUAGE", "RUNTIME", "OTHER"}
	desired := make(map[AnnotationSelection]pgtype.UUID, len(write.patch.Components))
	programs := 0
	for _, item := range write.patch.Components {
		id, err := parseUUID(item.ID)
		if err != nil || !slices.Contains(roles, item.Role) {
			return invalidAnnotation("a selected technology is invalid")
		}
		if _, duplicate := desired[item]; duplicate {
			return invalidAnnotation("technology assignments must be unique")
		}
		desired[item] = id
		if item.Role == "SITE_PROGRAM" {
			programs++
		}
	}
	if programs > 1 {
		return invalidAnnotation("a site can have at most one site program")
	}
	for _, item := range before {
		if _, exists := desired[item.AnnotationSelection]; exists {
			delete(desired, item.AnnotationSelection)
			continue
		}
		id, err := parseUUID(item.ID)
		if err != nil {
			return err
		}
		if err := write.q.UnassignSiteSoftwareComponent(ctx, dbgen.UnassignSiteSoftwareComponentParams{SiteID: write.row.ID, ComponentID: id, Role: item.Role}); err != nil {
			return err
		}
	}
	for item, id := range desired {
		component, err := write.q.GetSoftwareComponentByID(ctx, id)
		if isNotFound(err) || err == nil && !component.IsEnabled {
			return invalidAnnotation("a selected technology is unavailable")
		}
		if err != nil {
			return err
		}
		_, err = write.q.AssignSiteSoftwareComponent(ctx, dbgen.AssignSiteSoftwareComponentParams{
			SiteID: write.row.ID, ComponentID: id, Role: item.Role, EvidenceSource: "MANUAL",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
