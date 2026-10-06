package publicview

import (
	"context"
	"errors"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"reflect"
	"testing"
)

func newTestService(queries Queries) *Service { return New(queries, nil) }

type recordingMetrics struct{ queries, responses []string }

func (metrics *recordingMetrics) RecordQuery(_ context.Context, ids []string) {
	metrics.queries = append(metrics.queries, ids...)
}
func (metrics *recordingMetrics) RecordResponse(_ context.Context, ids []string) map[string]SiteMetrics {
	metrics.responses = append(metrics.responses, ids...)
	result := make(map[string]SiteMetrics)
	for _, id := range ids {
		result[id] = SiteMetrics{ClickCount: 12, QueryCount: 1, ResponseCount: 1}
	}
	return result
}

func TestDisplayMetricsDistinguishQueryFromFailedAssembly(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "assembly failure"}[failed], func(t *testing.T) {
			// Given
			row := testSite("A1b2C3d4E")
			stub := queryStub{count: 1, announcements: []dbgen.ContentAnnouncement{}, listRandom: func(context.Context, int32) ([]dbgen.DirectorySite, error) { return []dbgen.DirectorySite{row}, nil }}
			if failed {
				stub.announcementErr = errors.New("announcement lookup failed")
			}
			metrics := &recordingMetrics{}
			service := New(stub, metrics)
			// When
			result, err := service.Home(t.Context())
			// Then
			if !reflect.DeepEqual(metrics.queries, []string{row.ShortID}) {
				t.Fatalf("query metric = %v", metrics.queries)
			}
			if failed {
				if err == nil || len(metrics.responses) != 0 {
					t.Fatalf("failed view error=%v, responses=%v", err, metrics.responses)
				}
			} else if err != nil || !reflect.DeepEqual(metrics.responses, []string{row.ShortID}) || result.Sites[0].Metrics.ClickCount != 12 {
				t.Fatalf("view=%#v error=%v responses=%v", result, err, metrics.responses)
			}
		})
	}
}

func TestIconLookupDoesNotCountAsPublicDisplay(t *testing.T) {
	// Given
	row := testSite("A1b2C3d4E")
	metrics := &recordingMetrics{}
	service := New(queryStub{byShortID: row}, metrics)
	// When
	_, _ = service.SiteIconByIdentifier(t.Context(), SiteIdentifier{Kind: IdentifierShortID, Value: row.ShortID})
	// Then
	if len(metrics.queries) != 0 || len(metrics.responses) != 0 {
		t.Fatalf("icon counted: %#v", metrics)
	}
}

func TestImageMetadataDoesNotCountAsPublicDisplay(t *testing.T) {
	// Given
	row := testSite("A1b2C3d4E")
	metrics := &recordingMetrics{}
	service := New(queryStub{byShortID: row}, metrics)
	// When
	profile, err := service.SiteMetadataByIdentifier(t.Context(), SiteIdentifier{Kind: IdentifierShortID, Value: row.ShortID})
	// Then
	if err != nil || profile.ShortID != row.ShortID {
		t.Fatalf("metadata=%#v error=%v", profile, err)
	}
	if len(metrics.queries) != 0 || len(metrics.responses) != 0 {
		t.Fatalf("metadata counted: %#v", metrics)
	}
}
