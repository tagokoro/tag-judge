package usecase

import (
	"context"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type OAuthProvider interface {
	AuthCodeURL(state string) string
	ExchangeUser(ctx context.Context, code string) (OAuthUser, error)
}

type OAuthUser struct {
	ProviderUserID string
	Name           string
	Email          string
}

type AuthUsecase struct {
	repo       repository.AuthRepository
	google     OAuthProvider
	sessionTTL time.Duration
}

func NewAuthUsecase(repo repository.AuthRepository, google OAuthProvider, sessionTTL time.Duration) *AuthUsecase {
	return &AuthUsecase{
		repo:       repo,
		google:     google,
		sessionTTL: sessionTTL,
	}
}

func (u *AuthUsecase) GoogleLoginURL(state string) (string, error) {
	if u.google == nil {
		return "", apperror.Internal("google oauth is not configured")
	}
	return u.google.AuthCodeURL(state), nil
}

func (u *AuthUsecase) HandleGoogleCallback(ctx context.Context, code string) (auth.Session, error) {
	if u.google == nil {
		return auth.Session{}, apperror.Internal("google oauth is not configured")
	}
	if code == "" {
		return auth.Session{}, apperror.InvalidInput("INVALID_OAUTH_CODE", "oauth code is required")
	}

	googleUser, err := u.google.ExchangeUser(ctx, code)
	if err != nil {
		return auth.Session{}, err
	}

	actor, err := u.repo.UpsertGoogleUser(ctx, repository.GoogleUser{
		ProviderUserID: googleUser.ProviderUserID,
		Name:           googleUser.Name,
		Email:          googleUser.Email,
	})
	if err != nil {
		return auth.Session{}, err
	}

	token, err := auth.NewToken()
	if err != nil {
		return auth.Session{}, err
	}

	expiresAt := time.Now().Add(u.sessionTTL)
	if err := u.repo.CreateSession(ctx, repository.CreateSessionParams{
		UserID:    actor.UserID,
		TokenHash: auth.HashToken(token),
		ExpiresAt: expiresAt,
	}); err != nil {
		return auth.Session{}, err
	}

	return auth.Session{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (u *AuthUsecase) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return u.repo.RevokeSessionByTokenHash(ctx, auth.HashToken(token))
}
