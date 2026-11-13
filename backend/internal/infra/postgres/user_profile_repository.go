package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type UserProfileRepository struct {
	queries *sqlc.Queries
}

func NewUserProfileRepository(db *pgxpool.Pool) *UserProfileRepository {
	return &UserProfileRepository{queries: sqlc.New(db)}
}

// Create maps each unique constraint to its own conflict: the primary key
// means the user already chose a handle, the handle key means someone else
// uses it.
func (r *UserProfileRepository) Create(ctx context.Context, params repository.CreateUserProfileParams) (*domain.UserProfile, error) {
	row, err := r.queries.CreateUserProfile(ctx, sqlc.CreateUserProfileParams{
		UserID: params.UserID,
		Handle: params.Handle,
	})
	if err != nil {
		if isUniqueConstraintError(err, "user_profiles_pkey") {
			return nil, apperror.Conflict("HANDLE_ALREADY_SET", "you already have a handle")
		}
		if isUniqueConstraintError(err, "user_profiles_handle_unique") {
			return nil, apperror.Conflict("HANDLE_TAKEN", "handle is already taken")
		}
		// Only a development header actor can name a user that does not exist.
		if isForeignKeyConstraintError(err, "user_profiles_user_id_fkey") {
			return nil, apperror.Unauthorized("UNAUTHORIZED", "authentication required")
		}
		return nil, err
	}
	return newUserProfile(row.UserID, row.Handle, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserProfileRepository) FindByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	row, err := r.queries.GetUserProfileByUserID(ctx, userID)
	if err != nil {
		return nil, userProfileError(err)
	}
	return newUserProfile(row.UserID, row.Handle, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserProfileRepository) FindByHandle(ctx context.Context, handle string) (*domain.UserProfile, error) {
	row, err := r.queries.GetUserProfileByHandle(ctx, handle)
	if err != nil {
		return nil, userProfileError(err)
	}
	return newUserProfile(row.UserID, row.Handle, row.CreatedAt, row.UpdatedAt), nil
}

func userProfileError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.NotFound("USER_NOT_FOUND", "user not found")
	}
	return err
}

func newUserProfile(userID, handle string, createdAt, updatedAt pgtype.Timestamptz) *domain.UserProfile {
	return &domain.UserProfile{
		UserID:    userID,
		Handle:    handle,
		CreatedAt: createdAt.Time,
		UpdatedAt: updatedAt.Time,
	}
}

// isForeignKeyConstraintError reports whether err is the foreign key violation
// for the given constraint name.
func isForeignKeyConstraintError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23503" && pgErr.ConstraintName == constraintName
}
