package taskcenter

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
)

func csvText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) || strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	return value
}

func (r *Repository) Export(ctx context.Context, id, format string, q Query) ([]byte, error) {
	run, err := r.run(ctx, id)
	if err != nil {
		return nil, err
	}
	targets := []Target{}
	q.Page, q.PageSize = 1, 100
	for {
		page, err := r.Targets(ctx, id, q)
		if err != nil {
			return nil, err
		}
		targets = append(targets, page.Items...)
		if len(targets) >= page.Total {
			break
		}
		q.Page++
	}
	if format == "csv" {
		var out bytes.Buffer
		writer := csv.NewWriter(&out)
		if err := writer.Write([]string{"id", "site_id", "kind", "address", "state", "reason", "sample_started_at"}); err != nil {
			return nil, err
		}
		for _, t := range targets {
			at := ""
			if t.SampleStartedAt != nil {
				at = t.SampleStartedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			if err := writer.Write([]string{t.ID, t.SiteID, t.Kind, csvText(t.Address), t.State, csvText(t.Reason), at}); err != nil {
				return nil, err
			}
		}
		writer.Flush()
		return out.Bytes(), writer.Error()
	}
	// Ordinary configured headers can contain third-party identifiers; exports
	// preserve their names, never their values.
	headers := map[string]string{}
	for name := range run.Snapshot.Headers {
		headers[name] = "[redacted]"
	}
	run.Snapshot.Headers = headers
	return json.Marshal(struct {
		Run     Run      `json:"run"`
		Targets []Target `json:"targets"`
	}{run, targets})
}
