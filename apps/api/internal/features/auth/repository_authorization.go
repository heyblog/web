package auth

import (
	"context"
)

func (repo *repository) permissions(ctx context.Context, userID string) ([]Permission, error) {
	parsed, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := repo.queries.ListUserManagementPermissions(ctx, parsed)
	if err != nil {
		return nil, err
	}
	result := make([]Permission, len(rows))
	for index, key := range rows {
		result[index] = Permission(key)
	}
	return result, nil
}
