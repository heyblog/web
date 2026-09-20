package apikey

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "heyblog-api/internal/database/gen"
)

func isClientNameConflict(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" &&
		databaseError.ConstraintName == "api_clients_name_unique_idx"
}

func rotatedExpiration(audience Audience, previous dbgen.IdentityApiKey, now time.Time) pgtype.Timestamptz {
	if audience == AudienceInternal {
		return requiredTime(now.Add(maximumInternalKeyLife))
	}
	if !previous.ExpiresAt.Valid {
		return pgtype.Timestamptz{}
	}
	return requiredTime(now.Add(previous.ExpiresAt.Time.Sub(previous.CreatedAt.Time)))
}

func mapClient(row dbgen.IdentityApiClient, scopes []Scope, keys []Key) Client {
	return Client{
		ID: row.ID.String(), Name: row.Name, Description: row.Description, Audience: Audience(row.Audience),
		Scopes: scopes, DisabledAt: timeValue(row.DisabledAt), CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time, Keys: keys,
	}
}

func mapKey(row dbgen.IdentityApiKey) Key {
	return Key{
		ID: row.ID.String(), Prefix: row.KeyPrefix, ExpiresAt: timeValue(row.ExpiresAt),
		LastUsedAt: timeValue(row.LastUsedAt), CreatedAt: row.CreatedAt.Time,
		RevokedAt: timeValue(row.RevokedAt), RotatedFrom: optionalUUIDString(row.RotatedFromID),
	}
}

func mapListedKey(row dbgen.ListAPIKeysRow) Key {
	return Key{
		ID: row.ID.String(), Prefix: row.KeyPrefix, ExpiresAt: timeValue(row.ExpiresAt),
		LastUsedAt: timeValue(row.LastUsedAt), CreatedAt: row.CreatedAt.Time,
		RevokedAt: timeValue(row.RevokedAt), RotatedFrom: optionalUUIDString(row.RotatedFromID),
	}
}

func optionalUUIDString(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	result := value.String()
	return &result
}

func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return parsed, nil
}

func optionalTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return requiredTime(*value)
}

func requiredTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func timeValue(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
