package dataimport

import (
	"sort"
	"strings"
)

type componentCandidate struct {
	ID             string
	Name           string
	NormalizedName string
	Description    string
	HomepageURL    string
	RepositoryURL  string
	IsOpenSource   bool
	IsEnabled      bool
	Priority       int
}

func mergeComponentCandidates(candidates []componentCandidate) ComponentRow {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].Priority != candidates[right].Priority {
			return candidates[left].Priority > candidates[right].Priority
		}
		return candidates[left].ID < candidates[right].ID
	})
	canonical := candidates[0]
	row := ComponentRow{
		ID: canonical.ID, Name: strings.TrimSpace(canonical.Name), NormalizedName: canonical.NormalizedName,
		Description: canonical.Description, HomepageURL: canonical.HomepageURL,
		RepositoryURL: canonical.RepositoryURL, IsOpenSource: canonical.IsOpenSource,
		IsEnabled: canonical.IsEnabled,
	}
	for _, candidate := range candidates {
		if row.Description == "" && candidate.Description != "" {
			row.Description = candidate.Description
		}
		if row.HomepageURL == "" && candidate.HomepageURL != "" {
			row.HomepageURL = candidate.HomepageURL
		}
		if row.RepositoryURL == "" && candidate.RepositoryURL != "" {
			row.RepositoryURL = candidate.RepositoryURL
		}
	}
	return row
}
