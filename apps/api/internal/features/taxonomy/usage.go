package taxonomy

// Count each object once per tag, including tags used in both classification roles.
func annotateTagUsage(g *graph) {
	indexes := make(map[string]int, len(g.Tags))
	roles := make([]map[string]bool, len(g.Tags))
	for i := range g.Tags {
		indexes[g.Tags[i].ID] = i
		roles[i] = map[string]bool{}
		g.Tags[i].SiteCount = 0
		g.Tags[i].ArticleCount = 0
		g.Tags[i].Roles = []string{}
	}
	paths := make(map[string]Cascade, len(g.Cascades))
	for _, path := range g.Cascades {
		paths[path.ID] = path
		if path.MergedIntoID != "" {
			continue
		}
		if i, ok := indexes[path.PrimaryID]; ok {
			roles[i]["PRIMARY"] = true
		}
		if i, ok := indexes[path.SecondaryID]; ok {
			roles[i]["SECONDARY"] = true
		}
	}
	for _, object := range g.Objects {
		path := paths[object.CascadeID]
		used := make(map[string]bool, len(object.Tags)+2)
		used[path.PrimaryID] = true
		used[path.SecondaryID] = true
		for _, assignment := range object.Tags {
			used[assignment.TagID] = true
			if i, ok := indexes[assignment.TagID]; ok {
				roles[i][assignment.Role] = true
			}
		}
		for id := range used {
			i, ok := indexes[id]
			if !ok {
				continue
			}
			if object.Scope == "SITE" {
				g.Tags[i].SiteCount++
			} else {
				g.Tags[i].ArticleCount++
			}
		}
	}
	for i := range g.Tags {
		for _, role := range []string{"PRIMARY", "SECONDARY", "TERTIARY", "WARNING"} {
			if roles[i][role] {
				g.Tags[i].Roles = append(g.Tags[i].Roles, role)
			}
		}
	}
}
