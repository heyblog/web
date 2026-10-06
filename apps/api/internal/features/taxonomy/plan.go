package taxonomy

import "slices"

func buildPlan(current graph, input ChangeInput) (changePlan, error) {
	next := current
	next.Tags = slices.Clone(current.Tags)
	next.Cascades = slices.Clone(current.Cascades)
	next.Objects = slices.Clone(current.Objects)
	for i := range next.Objects {
		next.Objects[i].Tags = slices.Clone(current.Objects[i].Tags)
	}
	plan := changePlan{Preview: Preview{RetainedLabels: []RetainedLabel{}, Revision: current.Revision, Blockers: []string{}, Paths: []PathImpact{}}, Graph: next, Affected: map[string]bool{}, Merged: map[string]string{}}
	switch input.Kind {
	case "merge":
		plan.merge(input)
	case "path_update", "path_merge":
		plan.changePath(input)
	default:
		plan.Blockers = append(plan.Blockers, "invalid_change")
	}
	plan.coalescePaths(current)
	plan.collectPaths(current)
	if len(plan.Blockers) == 0 {
		plan.reconcile(current, input)
	}
	return finishPlan(plan, input)
}
func finishPlan(plan changePlan, input ChangeInput) (changePlan, error) {
	input.Fingerprint = ""
	value := struct {
		Revision string
		Input    ChangeInput
	}{plan.Revision, input}
	var err error
	plan.Fingerprint, err = digest(value)
	return plan, err
}
func protectedTag(tag Tag) bool { return tag.SystemKey == "other" }
func (p *changePlan) merge(input ChangeInput) {
	source, target := findTag(p.Graph, input.SourceID), findTag(p.Graph, input.TargetID)
	if source.ID == "" || target.ID == "" || source.ID == target.ID || !target.Enabled {
		p.Blockers = append(p.Blockers, "merge_target_required")
		return
	}
	if protectedTag(source) {
		p.Blockers = append(p.Blockers, "fallback_protected")
		return
	}
	p.Merged[source.ID] = target.ID
	for i := range p.Graph.Tags {
		if p.Graph.Tags[i].ID == target.ID {
			p.Graph.Tags[i].Labels = slices.Clone(p.Graph.Tags[i].Labels)
			for _, l := range source.Labels {
				l.TagID = target.ID
				p.Graph.Tags[i].Labels = append(p.Graph.Tags[i].Labels, l)
			}
		}
	}
	p.Graph.Tags = slices.DeleteFunc(p.Graph.Tags, func(t Tag) bool { return t.ID == source.ID })
	for i, c := range p.Graph.Cascades {
		if c.PrimaryID == source.ID {
			p.Graph.Cascades[i].PrimaryID = target.ID
		}
		if c.SecondaryID == source.ID {
			p.Graph.Cascades[i].SecondaryID = target.ID
		}
	}
}
func (p *changePlan) changePath(input ChangeInput) {
	source := findCascade(p.Graph, input.SourceID)
	if source.ID == "" || source.MergedIntoID != "" {
		p.Blockers = append(p.Blockers, "path_unavailable")
		return
	}
	if source.Key == "other/topic-other-other" {
		p.Blockers = append(p.Blockers, "fallback_protected")
		return
	}
	for i, c := range p.Graph.Cascades {
		if c.ID != source.ID {
			continue
		}
		if input.Kind == "path_merge" {
			target := findCascade(p.Graph, input.TargetID)
			if target.ID == "" || target.ID == source.ID || target.Scope != source.Scope || target.MergedIntoID != "" || !target.Enabled {
				p.Blockers = append(p.Blockers, "replacement_required")
				return
			}
			p.Graph.Cascades[i].MergedIntoID = target.ID
			p.Graph.Cascades[i].Enabled = false
		} else {
			primary, secondary := findTag(p.Graph, input.PrimaryID), findTag(p.Graph, input.SecondaryID)
			if primary.ID == "" || secondary.ID == "" || (input.Enabled && (!primary.Enabled || !secondary.Enabled)) {
				p.Blockers = append(p.Blockers, "path_tags_unavailable")
				return
			}
			p.Graph.Cascades[i].PrimaryID = primary.ID
			p.Graph.Cascades[i].SecondaryID = secondary.ID
			p.Graph.Cascades[i].Enabled = input.Enabled
		}
	}
}
func (p *changePlan) coalescePaths(before graph) {
	// Preserve the protected fallback, then prefer the existing stable path order.
	order := slices.Clone(p.Graph.Cascades)
	slices.SortStableFunc(order, func(a, b Cascade) int {
		if a.Key == "other/topic-other-other" && b.Key != "other/topic-other-other" {
			return -1
		}
		if b.Key == "other/topic-other-other" && a.Key != "other/topic-other-other" {
			return 1
		}
		oldA, oldB := findCascade(before, a.ID), findCascade(before, b.ID)
		changedA := oldA.PrimaryID != a.PrimaryID || oldA.SecondaryID != a.SecondaryID
		changedB := oldB.PrimaryID != b.PrimaryID || oldB.SecondaryID != b.SecondaryID
		if changedA && !changedB {
			return 1
		}
		if !changedA && changedB {
			return -1
		}
		return 0
	})
	pairs := map[string]string{}
	enabled := map[string]bool{}
	replacements := map[string]string{}
	for _, c := range order {
		if c.MergedIntoID != "" {
			continue
		}
		key := c.Scope + ":" + c.PrimaryID + ":" + c.SecondaryID
		enabled[key] = enabled[key] || c.Enabled
		if id, ok := pairs[key]; ok {
			replacements[c.ID] = id
		} else {
			pairs[key] = c.ID
		}
	}
	for i, c := range p.Graph.Cascades {
		if c.MergedIntoID == "" {
			p.Graph.Cascades[i].Enabled = enabled[c.Scope+":"+c.PrimaryID+":"+c.SecondaryID]
		}
		if id, ok := replacements[c.ID]; ok {
			p.Graph.Cascades[i].MergedIntoID = id
			p.Graph.Cascades[i].Enabled = false
		}
		if id, ok := replacements[c.MergedIntoID]; ok {
			p.Graph.Cascades[i].MergedIntoID = id
		}
	}
}
