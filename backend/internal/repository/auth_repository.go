package repository

import (
	"context"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/auth"
)

type GoogleUser struct {
	ProviderUserID string
	Name           string
	Email          string
}

// GoogleLogin is the user a Google login resolved to. HasProfile is false
// until the user creates their handle.
type GoogleLogin struct {
	Actor      auth.Actor
	HasProfile bool
}

type CreateSessionParams struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

type AuthRepository interface {
	UpsertGoogleUser(ctx context.Context, user GoogleUser) (GoogleLogin, error)
	CreateSession(ctx context.Context, params CreateSessionParams) error
	FindActorBySessionTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error)
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string) error
}
