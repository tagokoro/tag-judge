package repository

import (
	"context"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateOAuthClientParams struct {
	ID string
	// SecretHash is empty for a public client.
	SecretHash   string
	Name         string
	RedirectURIs []string
}

type CreateOAuthAuthorizationCodeParams struct {
	CodeHash      string
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	ExpiresAt     time.Time
}

type CreateOAuthTokenParams struct {
	ClientID         string
	UserID           string
	AccessTokenHash  string
	AccessExpiresAt  time.Time
	RefreshTokenHash string
	RefreshExpiresAt time.Time
}

type OAuthRepository interface {
	CreateClient(ctx context.Context, params CreateOAuthClientParams) error
	// FindClient returns NotFound for an unknown client.
	FindClient(ctx context.Context, id string) (*domain.OAuthClient, error)
	CreateAuthorizationCode(ctx context.Context, params CreateOAuthAuthorizationCodeParams) error
	// ConsumeAuthorizationCode marks the code used and returns it, or
	// NotFound when it is unknown, used, or expired.
	ConsumeAuthorizationCode(ctx context.Context, codeHash string) (*domain.OAuthAuthorizationCode, error)
	CreateToken(ctx context.Context, params CreateOAuthTokenParams) error
	// RotateToken revokes the pair holding refreshTokenHash and stores next
	// in its place, both or neither. It returns NotFound when the refresh
	// token is not a live one of clientID. next.UserID is filled from the
	// revoked pair.
	RotateToken(ctx context.Context, clientID, refreshTokenHash string, next CreateOAuthTokenParams) error
	// FindActorByAccessTokenHash returns Unauthorized for an unknown,
	// expired, or revoked token.
	FindActorByAccessTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error)
}
