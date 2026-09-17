//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func verifyTagAndIconConstraints(ctx context.Context, t *testing.T, connection *pgxpool.Pool) {
	t.Helper()

	siteID := insertSite(ctx, t, connection, "6Aa7Bb8Cc", "Tag and Icon", "tag-icon.example.com")
	otherSiteID := insertSite(ctx, t, connection, "7Aa8Bb9Cc", "Invalid Tag Role", "tag-role.example.com")
	var tertiaryTagID, warningTagID, alternateTertiaryTagID pgtype.UUID
	for _, tag := range []struct {
		name           string
		normalizedName string
		slug           string
		destination    *pgtype.UUID
	}{
		{name: "Technology", normalizedName: "technology", slug: "technology", destination: &tertiaryTagID},
		{name: "Sensitive", normalizedName: "sensitive", slug: "sensitive", destination: &warningTagID},
		{name: "Personal", normalizedName: "personal", slug: "personal", destination: &alternateTertiaryTagID},
	} {
		if err := connection.QueryRow(ctx, `
			INSERT INTO directory.tags (name, normalized_name, slug)
			VALUES ($1, $2, $3)
			RETURNING id
		`, tag.name, tag.normalizedName, tag.slug).Scan(tag.destination); err != nil {
			t.Fatalf("insert tag %q: %v", tag.name, err)
		}
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.tags (name, normalized_name, slug)
		VALUES ('Duplicate name', 'technology', 'different-slug')
	`); err == nil {
		t.Fatal("duplicate normalized tag name unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.tags (name, normalized_name, slug)
		VALUES ('Duplicate slug', 'different name', 'technology')
	`); err == nil {
		t.Fatal("duplicate tag slug unexpectedly succeeded")
	}
	firstPosition := int16(1)
	for _, assignment := range []struct {
		tagID    pgtype.UUID
		role     string
		note     *string
		position *int16
	}{
		{tagID: tertiaryTagID, role: "TERTIARY", position: &firstPosition},
		{tagID: warningTagID, role: "WARNING", note: stringPointer("Sensitive topic")},
	} {
		if _, err := connection.Exec(ctx, `
			INSERT INTO directory.site_tags (site_id, tag_id, role, assignment_source, note, position)
			VALUES ($1, $2, $3, 'SYSTEM', $4, $5)
		`, siteID, assignment.tagID, assignment.role, assignment.note, assignment.position); err != nil {
			t.Fatalf("assign %s tag: %v", assignment.role, err)
		}
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_tags (site_id, tag_id, role, position)
		VALUES ($1, $2, 'TERTIARY', 1)
	`, siteID, alternateTertiaryTagID); err == nil {
		t.Fatal("duplicate tertiary position unexpectedly succeeded")
	} else {
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "23505" ||
			databaseError.ConstraintName != "site_tags_tertiary_position_unique_idx" {
			t.Fatalf("duplicate tertiary position error = %v", err)
		}
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_tags (site_id, tag_id, role)
		VALUES ($1, $2, 'INVALID')
	`, otherSiteID, alternateTertiaryTagID); err == nil {
		t.Fatal("invalid tag role unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_tags (site_id, tag_id, role)
		VALUES ($1, $2, 'WARNING')
	`, siteID, tertiaryTagID); err == nil {
		t.Fatal("second role for one site tag unexpectedly succeeded")
	} else {
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "23505" ||
			databaseError.ConstraintName != "site_tags_pkey" {
			t.Fatalf("duplicate site tag error = %v, want SQLSTATE 23505 from site_tags_pkey", err)
		}
	}

	if _, err := connection.Exec(ctx, `
		INSERT INTO directory.site_icons (site_id, content, media_type, sha256)
		VALUES ($1, decode('00', 'hex'), 'image/png', decode(repeat('00', 32), 'hex'))
	`, siteID); err != nil {
		t.Fatalf("insert one-byte icon: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.site_icons SET content = decode(repeat('ab', 1048576), 'hex') WHERE site_id = $1
	`, siteID); err != nil {
		t.Fatalf("store one-MiB icon: %v", err)
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.site_icons SET content = ''::bytea WHERE site_id = $1
	`, siteID); err == nil {
		t.Fatal("empty icon unexpectedly succeeded")
	}
	if _, err := connection.Exec(ctx, `
		UPDATE directory.site_icons SET content = decode(repeat('ab', 1048577), 'hex') WHERE site_id = $1
	`, siteID); err == nil {
		t.Fatal("icon larger than one MiB unexpectedly succeeded")
	}
}
