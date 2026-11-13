package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateUserProfileParams struct {
	UserID string
	Handle string
}

type UserProfileRepository interface {
	Create(ctx context.Context, params CreateUserProfileParams) (*domain.UserProfile, error)
	FindByUserID(ctx context.Context, userID string) (*domain.UserProfile, error)
	FindByHandle(ctx context.Context, handle string) (*domain.UserProfile, error)
}
