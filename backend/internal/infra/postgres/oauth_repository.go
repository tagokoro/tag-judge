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

type OAuthRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewOAuthRepository(db *pgxpool.Pool) *OAuthRepository {
	return &OAuthRepository{db: db, queries: sqlc.New(db)}
}

func (r *OAuthRepository) CreateClient(ctx context.Context, params repository.CreateOAuthClientParams) error {
	return r.queries.CreateOAuthClient(ctx, sqlc.CreateOAuthClientParams{
		ID:           params.ID,
		SecretHash:   pgtype.Text{String: params.SecretHash, Valid: params.SecretHash != ""},
		Name:         params.Name,
		RedirectUris: params.RedirectURIs,
	})
}

func (r *OAuthRepository) FindClient(ctx context.Context, id string) (*domain.OAuthClient, error) {
	row, err := r.queries.GetOAuthClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("OAUTH_CLIENT_NOT_FOUND", "oauth client not found")
	}
	if err != nil {
		return nil, err
	}
	return &domain.OAuthClient{
		ID:           row.ID,
		SecretHash:   row.SecretHash.String,
		Name:         row.Name,
		RedirectURIs: row.RedirectUris,
		CreatedAt:    row.CreatedAt.Time,
	}, nil
}

func (r *OAuthRepository) CreateAuthorizationCode(ctx context.Context, params repository.CreateOAuthAuthorizationCodeParams) error {
	return r.queries.CreateOAuthAuthorizationCode(ctx, sqlc.CreateOAuthAuthorizationCodeParams{
		CodeHash:      params.CodeHash,
		ClientID:      params.ClientID,
		UserID:        params.UserID,
		RedirectUri:   params.RedirectURI,
		CodeChallenge: params.CodeChallenge,
		ExpiresAt:     pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
	})
}

func (r *OAuthRepository) ConsumeAuthorizationCode(ctx context.Context, codeHash string) (*domain.OAuthAuthorizationCode, error) {
	row, err := r.queries.ConsumeOAuthAuthorizationCode(ctx, codeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("OAUTH_CODE_NOT_FOUND", "authorization code is invalid, used, or expired")
	}
	if err != nil {
		return nil, err
	}
	return &domain.OAuthAuthorizationCode{
		ClientID:      row.ClientID,
		UserID:        row.UserID,
		RedirectURI:   row.RedirectUri,
		CodeChallenge: row.CodeChallenge,
	}, nil
}

func (r *OAuthRepository) CreateToken(ctx context.Context, params repository.CreateOAuthTokenParams) error {
	return r.queries.CreateOAuthToken(ctx, oauthTokenParams(params))
}

func (r *OAuthRepository) RotateToken(ctx context.Context, clientID, refreshTokenHash string, next repository.CreateOAuthTokenParams) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	userID, err := queries.RevokeOAuthTokenByRefreshHash(ctx, sqlc.RevokeOAuthTokenByRefreshHashParams{
		RefreshTokenHash: refreshTokenHash,
		ClientID:         clientID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.NotFound("OAUTH_REFRESH_TOKEN_NOT_FOUND", "refresh token is invalid, used, or expired")
	}
	if err != nil {
		return err
	}
	next.ClientID = clientID
	next.UserID = userID
	if err := queries.CreateOAuthToken(ctx, oauthTokenParams(next)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *OAuthRepository) FindActorByAccessTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error) {
	row, err := r.queries.UseOAuthAccessToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Actor{}, apperror.Unauthorized("INVALID_TOKEN", "access token is invalid, expired, or revoked")
	}
	if err != nil {
		return auth.Actor{}, err
	}
	return auth.Actor{
		UserID: row.UserID,
		Role:   domain.UserRole(row.Role),
	}, nil
}

func oauthTokenParams(params repository.CreateOAuthTokenParams) sqlc.CreateOAuthTokenParams {
	return sqlc.CreateOAuthTokenParams{
		ClientID:         params.ClientID,
		UserID:           params.UserID,
		AccessTokenHash:  params.AccessTokenHash,
		AccessExpiresAt:  pgtype.Timestamptz{Time: params.AccessExpiresAt, Valid: true},
		RefreshTokenHash: params.RefreshTokenHash,
		RefreshExpiresAt: pgtype.Timestamptz{Time: params.RefreshExpiresAt, Valid: true},
	}
}
