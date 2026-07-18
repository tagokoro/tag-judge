package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type AuthRepository struct {
	queries *sqlc.Queries
}

func NewAuthRepository(db *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{queries: sqlc.New(db)}
}

func (r *AuthRepository) UpsertGoogleUser(ctx context.Context, user repository.GoogleUser) (auth.Actor, error) {
	row, err := r.queries.UpsertGoogleUser(ctx, sqlc.UpsertGoogleUserParams{
		ProviderUserID: user.ProviderUserID,
		Name:           user.Name,
		Email:          user.Email,
	})
	if err != nil {
		return auth.Actor{}, err
	}

	return auth.Actor{
		UserID: row.UserID,
		Role:   domain.UserRole(row.Role),
	}, nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, params repository.CreateSessionParams) error {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return apperror.InvalidInput("INVALID_USER_ID", "user_id must be a valid UUID")
	}

	return r.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:    userID,
		TokenHash: params.TokenHash,
		ExpiresAt: pgtype.Timestamptz{
			Time:  params.ExpiresAt,
			Valid: true,
		},
	})
}

func (r *AuthRepository) FindActorBySessionTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error) {
	row, err := r.queries.GetActorBySessionTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Actor{}, apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	if err != nil {
		return auth.Actor{}, err
	}

	return auth.Actor{
		UserID: row.UserID,
		Role:   domain.UserRole(row.Role),
	}, nil
}

func (r *AuthRepository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return r.queries.RevokeSessionByTokenHash(ctx, tokenHash)
}

func parseUUID(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return uuid, nil
}
