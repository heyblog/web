package taxonomy

import (
	"reflect"
	"slices"
)

func (p *changePlan) reconcile(before graph, input ChangeInput) {
	for i, o := range p.Graph.Objects {
		oldCascade := findCascade(before, o.CascadeID)
		cascade := findCascade(p.Graph, o.CascadeID)
		for cascade.MergedIntoID != "" {
			cascade = findCascade(p.Graph, cascade.MergedIntoID)
		}
		p.Graph.Objects[i].CascadeID = cascade.ID
		if oldCascade.PrimaryID != cascade.PrimaryID && (input.Kind != "merge" || oldCascade.PrimaryID != input.SourceID || cascade.PrimaryID != input.TargetID) {
			p.Graph.Objects[i].PrimaryLabelID = findTag(p.Graph, cascade.PrimaryID).DefaultLabelID
		}
		if oldCascade.SecondaryID != cascade.SecondaryID && (input.Kind != "merge" || oldCascade.SecondaryID != input.SourceID || cascade.SecondaryID != input.TargetID) {
			p.Graph.Objects[i].SecondaryLabelID = findTag(p.Graph, cascade.SecondaryID).DefaultLabelID
		}
		ordered := slices.Clone(o.Tags)
		slices.SortStableFunc(ordered, func(a, b assignment) int {
			if input.Kind == "merge" {
				if a.TagID == input.TargetID && b.TagID == input.SourceID {
					return -1
				}
				if b.TagID == input.TargetID && a.TagID == input.SourceID {
					return 1
				}
			}
			return 0
		})
		merged := make([]assignment, 0, len(o.Tags))
		for _, a := range ordered {
			if input.Kind == "merge" && a.TagID == input.SourceID {
				a.TagID = input.TargetID
			}
			if a.Role == "TERTIARY" && (a.TagID == cascade.PrimaryID || a.TagID == cascade.SecondaryID) {
				p.RemovedDuplicates++
				continue
			}
			duplicate := -1
			for j, t := range merged {
				if t.TagID == a.TagID {
					duplicate = j
					break
				}
			}
			if duplicate >= 0 {
				existing := merged[duplicate]
				if existing.Role != a.Role || existing.Source != a.Source || !reflect.DeepEqual(existing.Note, a.Note) {
					p.Blockers = append(p.Blockers, "assignment_conflict:"+o.Scope+":"+o.ID)
					continue
				}
				p.RemovedDuplicates++
				continue
			}
			merged = append(merged, a)
		}
		slices.SortStableFunc(merged, func(a, b assignment) int { return int(a.Position) - int(b.Position) })
		var position int16
		for j := range merged {
			if merged[j].Role == "TERTIARY" {
				position++
				merged[j].Position = position
			}
		}
		p.Graph.Objects[i].Tags = merged
		for _, a := range merged {
			if input.Kind == "merge" && a.TagID == input.TargetID {
				name := ""
				for _, l := range findTag(p.Graph, a.TagID).Labels {
					if l.ID == a.LabelID {
						name = l.Name
					}
				}
				p.RetainedLabels = append(p.RetainedLabels, RetainedLabel{ObjectID: o.ID, Scope: o.Scope, TagID: a.TagID, LabelID: a.LabelID, Name: name})
			}
		}
		if oldCascade.PrimaryID != cascade.PrimaryID || oldCascade.SecondaryID != cascade.SecondaryID || o.CascadeID != cascade.ID || !reflect.DeepEqual(o.Tags, merged) {
			p.Affected[objectKey(o)] = true
			if o.Scope == "SITE" {
				p.SiteCount++
			} else {
				p.ArticleCount++
			}
		}
	}
}

func (p *changePlan) collectPaths(before graph) {
	seen := map[string]bool{}
	for _, path := range p.Paths {
		seen[path.CascadeID] = true
	}
	for _, c := range p.Graph.Cascades {
		old := findCascade(before, c.ID)
		if old.ID == "" || seen[c.ID] {
			continue
		}
		if old.PrimaryID != c.PrimaryID || old.SecondaryID != c.SecondaryID || old.MergedIntoID != c.MergedIntoID || old.Enabled != c.Enabled {
			p.Paths = append(p.Paths, PathImpact{CascadeID: c.ID, Scope: c.Scope, Label: findTag(before, old.PrimaryID).Name + " / " + findTag(before, old.SecondaryID).Name})
		}
	}
}
