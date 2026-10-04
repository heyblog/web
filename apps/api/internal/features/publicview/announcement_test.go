package publicview

import (
	"context"
	dbgen "heyblog-api/internal/infrastructure/database/gen"
	"testing"
	"time"
)

func TestHomeReturnsAllActiveMainAnnouncementsInQueryOrder(t *testing.T) {
	// Given multiple main announcements ordered by the authoritative query.
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	body := "**Full body**"
	service := New(queryStub{announcements: []dbgen.ContentAnnouncement{
		{Title: "First", Kind: "MAIN", Status: "PUBLISHED", ActionType: "NONE", Priority: 10, StartsAt: timestamp(start), BodyMarkdown: &body},
		{Title: "Second", Kind: "MAIN", Status: "PUBLISHED", ActionType: "NONE", Priority: 5, StartsAt: timestamp(start.Add(time.Hour))},
	}})
	// When the home read model is assembled.
	view, err := service.Home(context.Background())
	// Then no announcement is discarded and full content remains available.
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Announcements) != 2 || view.Announcements[0].Title != "First" || view.Announcements[1].Title != "Second" || view.Announcements[0].BodyMarkdown == nil || *view.Announcements[0].BodyMarkdown != body {
		t.Fatalf("announcements = %#v", view.Announcements)
	}
}
