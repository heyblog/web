package publicview

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"heyblog-api/internal/apperror"
	dbgen "heyblog-api/internal/database/gen"
)

type iconQueries struct {
	queryStub
	icon    dbgen.DirectorySiteIcon
	iconErr error
}

func (stub queryStub) GetSiteIcon(context.Context, pgtype.UUID) (dbgen.DirectorySiteIcon, error) {
	return dbgen.DirectorySiteIcon{}, pgx.ErrNoRows
}

func (stub queryStub) GetSiteIconHash(context.Context, pgtype.UUID) ([]byte, error) {
	return nil, pgx.ErrNoRows
}

func (stub iconQueries) GetSiteIcon(context.Context, pgtype.UUID) (dbgen.DirectorySiteIcon, error) {
	return stub.icon, stub.iconErr
}

func (stub iconQueries) GetSiteIconHash(context.Context, pgtype.UUID) ([]byte, error) {
	return stub.icon.Sha256, stub.iconErr
}

func TestSiteProfileIconHash(t *testing.T) {
	t.Parallel()
	for _, present := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "present"}[present], func(t *testing.T) {
			// Given an optional cached icon.
			queries := iconQueries{queryStub: queryStub{byShortID: testSite("A1b2C3d4E")}, iconErr: pgx.ErrNoRows}
			if present {
				queries.icon.Sha256 = bytes.Repeat([]byte{0xab}, 32)
				queries.iconErr = nil
			}
			// When reading the profile.
			profile, err := New(queries).SiteByIdentifier(t.Context(), SiteIdentifier{Kind: IdentifierShortID, Value: "A1b2C3d4E"})
			// Then only its digest is exposed, with null for missing icons.
			if err != nil {
				t.Fatal(err)
			}
			if present && (profile.IconHash == nil || *profile.IconHash != hex.EncodeToString(queries.icon.Sha256)) {
				t.Fatalf("hash=%v", profile.IconHash)
			}
			if !present && profile.IconHash != nil {
				t.Fatalf("hash=%v", profile.IconHash)
			}
		})
	}
}

func TestSiteIconStatusAndNormalization(t *testing.T) {
	t.Parallel()
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 256, 128))); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, visibility string
		content          []byte
		queryErr         error
		want             apperror.Kind
	}{
		{name: "visible", visibility: "VISIBLE", content: source.Bytes()},
		{name: "hidden", visibility: "HIDDEN", content: source.Bytes()},
		{name: "removed", visibility: "REMOVED", content: source.Bytes(), want: apperror.KindNotFound},
		{name: "missing icon", visibility: "VISIBLE", queryErr: pgx.ErrNoRows, want: apperror.KindNotFound},
		{name: "corrupt icon", visibility: "VISIBLE", content: []byte("bad"), want: apperror.KindNotFound},
		{name: "database failure", visibility: "VISIBLE", queryErr: errors.New("database unavailable"), want: apperror.KindUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a site and cached icon state.
			row := testSite("A1b2C3d4E")
			row.Visibility = test.visibility
			queries := iconQueries{queryStub: queryStub{byShortID: row}, icon: dbgen.DirectorySiteIcon{Content: test.content, Sha256: bytes.Repeat([]byte{0xcd}, 32)}, iconErr: test.queryErr}
			// When reading its icon.
			icon, err := New(queries).SiteIconByIdentifier(t.Context(), SiteIdentifier{Kind: IdentifierShortID, Value: row.ShortID})
			// Then the status and successful PNG follow the public contract.
			if test.want != "" {
				var appErr *apperror.Error
				if !errors.As(err, &appErr) || appErr.Kind() != test.want {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			config, err := png.DecodeConfig(bytes.NewReader(icon.Content))
			if err != nil || config.Width != 128 || config.Height != 64 {
				t.Fatalf("config=%+v err=%v", config, err)
			}
			if icon.Hash != hex.EncodeToString(queries.icon.Sha256) {
				t.Fatalf("hash=%s", icon.Hash)
			}
		})
	}
}
