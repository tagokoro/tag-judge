package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func TestHandleGoogleCallback(t *testing.T) {
	repo := &fakeAuthRepository{}
	provider := &fakeOAuthProvider{}
	uc := NewAuthUsecase(repo, provider, time.Hour)

	session, err := uc.HandleGoogleCallback(context.Background(), "oauth-code")
	if err != nil {
		t.Fatalf("HandleGoogleCallback returned error: %v", err)
	}

	if provider.code != "oauth-code" {
		t.Fatalf("provider code = %q, want oauth-code", provider.code)
	}
	if repo.googleUser.ProviderUserID != "google-sub" {
		t.Fatalf("provider user id = %q", repo.googleUser.ProviderUserID)
	}
	if session.Token == "" {
		t.Fatal("session token is empty")
	}
	if repo.session.TokenHash != auth.HashToken(session.Token) {
		t.Fatal("stored token hash should match returned raw token")
	}
	if !repo.session.ExpiresAt.After(time.Now()) {
		t.Fatal("session expiration should be in the future")
	}
}

func TestLogout(t *testing.T) {
	repo := &fakeAuthRepository{}
	uc := NewAuthUsecase(repo, nil, time.Hour)

	if err := uc.Logout(context.Background(), "session-token"); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}

	if repo.revokedTokenHash != auth.HashToken("session-token") {
		t.Fatal("revoked token hash should match raw token")
	}
}

type fakeOAuthProvider struct {
	code string
}

func (p *fakeOAuthProvider) AuthCodeURL(state string) string {
	return "/oauth?state=" + state
}

func (p *fakeOAuthProvider) ExchangeUser(_ context.Context, code string) (OAuthUser, error) {
	p.code = code
	return OAuthUser{
		ProviderUserID: "google-sub",
		Name:           "Google User",
		Email:          "google@example.com",
	}, nil
}

type fakeAuthRepository struct {
	googleUser       repository.GoogleUser
	session          repository.CreateSessionParams
	revokedTokenHash string
}

func (r *fakeAuthRepository) UpsertGoogleUser(_ context.Context, user repository.GoogleUser) (auth.Actor, error) {
	r.googleUser = user
	return auth.Actor{
		UserID: "00000000-0000-4000-8000-000000000001",
		Role:   domain.UserRoleUser,
	}, nil
}

func (r *fakeAuthRepository) CreateSession(_ context.Context, params repository.CreateSessionParams) error {
	r.session = params
	return nil
}

func (r *fakeAuthRepository) FindActorBySessionTokenHash(_ context.Context, _ string) (auth.Actor, error) {
	return auth.Actor{}, nil
}

func (r *fakeAuthRepository) RevokeSessionByTokenHash(_ context.Context, tokenHash string) error {
	r.revokedTokenHash = tokenHash
	return nil
}
