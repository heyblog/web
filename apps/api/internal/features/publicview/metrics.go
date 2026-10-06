package publicview

import (
	"context"
)

type SiteMetrics struct {
	ClickCount      int64 `json:"clickCount"`
	ImpressionCount int64 `json:"impressionCount"`
	ResponseCount   int64 `json:"responseCount"`
	QueryCount      int64 `json:"queryCount"`
}

type MetricsRecorder interface {
	RecordQuery(context.Context, []string)
	RecordResponse(context.Context, []string) map[string]SiteMetrics
}

func (service *Service) attachMetrics(ctx context.Context, cards []HomeSiteCard) {
	if service.metrics == nil {
		return
	}
	counts := service.metrics.RecordResponse(ctx, cardShortIDs(cards))
	for i := range cards {
		cards[i].Metrics = counts[cards[i].ShortID]
	}
}

func cardShortIDs(cards []HomeSiteCard) []string {
	ids := make([]string, len(cards))
	for i, card := range cards {
		ids[i] = card.ShortID
	}
	return ids
}
