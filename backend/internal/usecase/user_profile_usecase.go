package usecase

import (
	"context"
	"errors"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type UserProfileUsecase struct {
	profiles repository.UserProfileRepository
}

func NewUserProfileUsecase(profiles repository.UserProfileRepository) *UserProfileUsecase {
	return &UserProfileUsecase{profiles: profiles}
}

// CreateMine gives the actor their handle, once. The handle is their only name
// and never changes afterwards.
func (u *UserProfileUsecase) CreateMine(ctx context.Context, actor auth.Actor, handle string) (*domain.UserProfile, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if !domain.ValidHandle(handle) {
		return nil, apperror.InvalidInput("INVALID_HANDLE", "handle must be 2-16 lowercase letters, digits, or single hyphens")
	}
	return u.profiles.Create(ctx, repository.CreateUserProfileParams{
		UserID: actor.UserID,
		Handle: handle,
	})
}

// FindMine returns the actor's profile, or nil while they have no handle.
func (u *UserProfileUsecase) FindMine(ctx context.Context, actor auth.Actor) (*domain.UserProfile, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	profile, err := u.profiles.FindByUserID(ctx, actor.UserID)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, nil
	}
	return profile, err
}

func (u *UserProfileUsecase) GetByHandle(ctx context.Context, handle string) (*domain.UserProfile, error) {
	return u.profiles.FindByHandle(ctx, handle)
}
