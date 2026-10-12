package auth

import (
	"context"
	"errors"
	"time"

	dbgen "heyblog-api/internal/infrastructure/database/gen"

	"github.com/jackc/pgx/v5"
)

type managedMutation struct {
	Actor                                           User
	ID, UpdatedAt, Kind, DisplayName, Email, Status string
	Role                                            Role
	Permissions                                     []Permission
}

func (repo *repository) managedUsers(ctx context.Context, filter ManagedUsersFilter) (ManagedUsersPage, error) {
	total, err := repo.queries.CountManagedUsers(ctx, dbgen.CountManagedUsersParams{Search: filter.Search, Role: filter.Role, Status: filter.Status})
	if err != nil {
		return ManagedUsersPage{}, err
	}
	lastPage := max(int64(1), (total+int64(filter.PageSize)-1)/int64(filter.PageSize))
	filter.Page = min(filter.Page, int32(min(lastPage, 1_000_000)))
	rows, err := repo.queries.ListManagedUsersPage(ctx, dbgen.ListManagedUsersPageParams{Search: filter.Search, Role: filter.Role, Status: filter.Status, PageSize: filter.PageSize, PageOffset: (filter.Page - 1) * filter.PageSize})
	if err != nil {
		return ManagedUsersPage{}, err
	}
	users := make([]User, 0, len(rows))
	for _, row := range rows {
		permissions := make([]Permission, len(row.Permissions))
		for i, p := range row.Permissions {
			permissions[i] = Permission(p)
		}
		users = append(users, userDTO(mapUser(row.IdentityUser), permissions, row.HasGithub))
	}
	return ManagedUsersPage{Users: users, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func lockedActor(ctx context.Context, queries *dbgen.Queries, actor User) (User, error) {
	id, err := parseUUID(actor.ID)
	if err != nil {
		return User{}, err
	}
	row, err := queries.GetUserForUpdate(ctx, id)
	if err != nil {
		return User{}, err
	}
	record := mapUser(row)
	if !isActive(record) || record.DeletionRequestedAt != nil || record.AuthVersion != actor.AuthVersion || record.ProfileConfirmationStatus == "PENDING" {
		return User{}, newAuthError("unauthenticated", 401, "authentication is required")
	}
	permissions, err := queries.ListUserManagementPermissions(ctx, id)
	if err != nil {
		return User{}, err
	}
	keys := make([]Permission, len(permissions))
	for i, p := range permissions {
		keys[i] = Permission(p)
	}
	current := userDTO(record, keys, false)
	if !canManageUsers(current) {
		return User{}, newAuthError("forbidden", 403, "user management permission is required")
	}
	return current, nil
}

func (repo *repository) createManagedUser(ctx context.Context, actor User, profile ProfileConfirmation, email string) (dbUser, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return dbUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	if _, err := lockedActor(ctx, q, actor); err != nil {
		return dbUser{}, err
	}
	row, err := q.CreateUser(ctx, dbgen.CreateUserParams{Username: profile.Username, DisplayName: profile.DisplayName, Email: email})
	if err != nil {
		return dbUser{}, err
	}
	return mapUser(row), tx.Commit(ctx)
}

func (repo *repository) mutateManagedUser(ctx context.Context, change managedMutation) (dbUser, error) {
	if change.Actor.ID == change.ID {
		return dbUser{}, newAuthError("self_management_forbidden", 409, "use personal account settings for your own account")
	}
	id, err := parseUUID(change.ID)
	if err != nil {
		return dbUser{}, newAuthError("user_not_found", 404, "user was not found")
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return dbUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.queries.WithTx(tx)
	// Serializing management mutations also fixes the actor/target lock order when administrators edit one another.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(78231001)"); err != nil {
		return dbUser{}, err
	}
	actor, err := lockedActor(ctx, q, change.Actor)
	if err != nil {
		return dbUser{}, err
	}
	row, err := q.GetUserForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbUser{}, newAuthError("user_not_found", 404, "user was not found")
	}
	if err != nil {
		return dbUser{}, err
	}
	target := mapUser(row)
	if target.Email == nil || target.DeletionRequestedAt != nil {
		return dbUser{}, newAuthError("account_unavailable", 409, "account is unavailable")
	}
	if change.UpdatedAt != "" && change.UpdatedAt != target.UpdatedAt.UTC().Format(time.RFC3339Nano) {
		return dbUser{}, newAuthError("user_conflict", 409, "account changed; reload its current details")
	}
	if err := authorizeManagedMutation(actor, target, change); err != nil {
		return dbUser{}, err
	}
	updated, err := applyManagedMutation(ctx, q, row, change)
	if err != nil {
		return dbUser{}, identityError(err)
	}
	return mapUser(updated), tx.Commit(ctx)
}

func authorizeManagedMutation(actor User, target dbUser, change managedMutation) error {
	if change.Kind == "role" {
		if actor.Role != RoleSysAdmin {
			return newAuthError("forbidden", 403, "system administrator role is required")
		}
		if change.Role != RoleUser && change.Role != RoleAdmin {
			return newAuthError("invalid_role", 422, "role is invalid")
		}
		return nil
	}
	if change.Kind == "permissions" {
		return validatePermissions(actor, target, change.Permissions)
	}
	if actor.Role != RoleSysAdmin && target.Role != string(RoleUser) {
		return newAuthError("forbidden", 403, "this account requires a system administrator")
	}
	if (change.Kind == "password-reset" || change.Kind == "verification") && !isActive(target) {
		return newAuthError("account_unavailable", 409, "activate the account before sending account email")
	}
	return nil
}

func applyManagedMutation(ctx context.Context, q *dbgen.Queries, row dbgen.IdentityUser, change managedMutation) (dbgen.IdentityUser, error) {
	var err error
	switch change.Kind {
	case "profile":
		if row.DisplayName == change.DisplayName && row.Email != nil && *row.Email == change.Email {
			return row, nil
		}
		if row.Email == nil || *row.Email != change.Email {
			if err := q.DeleteEmailVerificationCodes(ctx, row.ID); err != nil {
				return row, err
			}
			if err := q.DeletePasswordResetTokens(ctx, row.ID); err != nil {
				return row, err
			}
		}
		return q.SetManagedProfile(ctx, dbgen.SetManagedProfileParams{ID: row.ID, DisplayName: change.DisplayName, Email: &change.Email})
	case "status":
		if row.AccessStatus == change.Status {
			return row, nil
		}
		if change.Status == "ACTIVE" {
			return q.ActivateUser(ctx, row.ID)
		}
		return q.SuspendUser(ctx, row.ID)
	case "sessions/revoke":
		err = q.BumpUserAuthVersion(ctx, row.ID)
	case "role":
		if row.Role == string(change.Role) {
			return row, nil
		}
		err = q.SetUserRole(ctx, dbgen.SetUserRoleParams{ID: row.ID, Role: string(change.Role)})
		if err == nil && change.Role == RoleUser {
			err = q.DeleteUserManagementPermissions(ctx, row.ID)
		}
	case "permissions":
		if err = q.DeleteUserManagementPermissions(ctx, row.ID); err != nil {
			return row, err
		}
		actorID, parseErr := parseUUID(change.Actor.ID)
		if parseErr != nil {
			return row, parseErr
		}
		for _, permission := range change.Permissions {
			if err = q.CreateUserManagementPermission(ctx, dbgen.CreateUserManagementPermissionParams{UserID: row.ID, PermissionKey: string(permission), GrantedBy: actorID}); err != nil {
				return row, err
			}
		}
		err = q.BumpUserAuthVersion(ctx, row.ID)
	case "password-reset", "verification":
		return row, nil
	default:
		return row, newAuthError("invalid_operation", 422, "account operation is invalid")
	}
	if err != nil {
		return row, err
	}
	return q.GetUserByID(ctx, row.ID)
}
