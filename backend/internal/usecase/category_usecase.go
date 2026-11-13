package usecase

import (
	"context"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// maxPagingOffset is the largest offset the list queries can represent: their
// sqlc params cast the value to int32, so anything larger would wrap (for
// example 4294967297 becomes 1). Validate before the repository is reached.
const maxPagingOffset = 1<<31 - 1

// validatePaging applies the shared list contract: default limit 20, limits
// from 1 through 100, and an offset that fits the int32 list queries. The exact
// spec §10 message is kept for negative offsets; a too-large non-negative
// offset gets a distinct stable 400 before it can wrap to a different page.
func validatePaging(limit int, limitProvided bool, offset int) (int, int, error) {
	if limitProvided && (limit <= 0 || limit > 100) {
		return 0, 0, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if offset < 0 {
		return 0, 0, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}
	if offset > maxPagingOffset {
		return 0, 0, apperror.InvalidInput("INVALID_OFFSET", "offset is too large")
	}
	if !limitProvided {
		limit = 20
	}
	return limit, offset, nil
}

const (
	maxCategoryNameLength        = 64
	maxCategoryDescriptionLength = 500
)

type CategoryUsecase struct {
	categories repository.CategoryRepository
	profiles   repository.UserProfileRepository
}

func NewCategoryUsecase(categories repository.CategoryRepository, profiles repository.UserProfileRepository) *CategoryUsecase {
	return &CategoryUsecase{categories: categories, profiles: profiles}
}

type UpdateCategoryInput struct {
	Name        *string
	Description *string
}

// ListMine lists every category the actor owns, including hidden ones. A user
// owns at most domain.MaxCategoriesPerUser, so one page holds them all.
func (u *CategoryUsecase) ListMine(ctx context.Context, actor auth.Actor) ([]domain.Category, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	return u.categories.ListByOwner(ctx, repository.ListCategoriesParams{
		OwnerUserID:   actor.UserID,
		IncludeHidden: true,
		Limit:         domain.MaxCategoriesPerUser,
	})
}

// ListByHandle lists a user's categories: all of them for the user themself,
// only those with a public library for everyone else.
func (u *CategoryUsecase) ListByHandle(ctx context.Context, actor auth.Actor, handle string, limit int, limitProvided bool, offset int) ([]domain.Category, error) {
	profile, err := u.profiles.FindByHandle(ctx, handle)
	if err != nil {
		return nil, err
	}
	limit, offset, err = validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	return u.categories.ListByOwner(ctx, repository.ListCategoriesParams{
		OwnerUserID:   profile.UserID,
		IncludeHidden: actor.UserID != "" && actor.UserID == profile.UserID,
		Limit:         limit,
		Offset:        offset,
	})
}

// Get returns a category the actor may see. A category without a public
// library is indistinguishable from a missing one for everyone but its owner.
func (u *CategoryUsecase) Get(ctx context.Context, actor auth.Actor, categoryID string) (*domain.Category, error) {
	category, err := u.find(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if !category.VisibleTo(actor.UserID) {
		return nil, categoryNotFound()
	}
	return category, nil
}

func (u *CategoryUsecase) Update(ctx context.Context, actor auth.Actor, categoryID string, input UpdateCategoryInput) (*domain.Category, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if input.Name == nil && input.Description == nil {
		return nil, apperror.InvalidInput("EMPTY_UPDATE", "at least one field is required")
	}
	if input.Name != nil {
		name, err := normalizeCategoryName(*input.Name)
		if err != nil {
			return nil, err
		}
		input.Name = &name
	}
	if input.Description != nil {
		if err := validateCategoryDescription(*input.Description); err != nil {
			return nil, err
		}
	}
	category, err := u.authorizeOwner(ctx, actor, categoryID)
	if err != nil {
		return nil, err
	}
	updated, err := u.categories.Update(ctx, repository.UpdateCategoryParams{
		ID:          category.ID,
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return nil, err
	}
	updated.OwnerHandle = category.OwnerHandle
	updated.HasPublicLibrary = category.HasPublicLibrary
	return updated, nil
}

// find resolves a public id. A malformed id cannot exist, so it shares the
// not-found contract with a missing category.
func (u *CategoryUsecase) find(ctx context.Context, categoryID string) (*domain.Category, error) {
	if !domain.ValidPublicID(categoryID) {
		return nil, categoryNotFound()
	}
	return u.categories.FindByPublicID(ctx, categoryID)
}

// authorizeOwner is the write gate for category changes: 403 for a category
// the actor can see but does not own, 404 for one hidden from them.
func (u *CategoryUsecase) authorizeOwner(ctx context.Context, actor auth.Actor, categoryID string) (*domain.Category, error) {
	category, err := u.find(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if category.OwnerUserID != actor.UserID {
		if !category.VisibleTo(actor.UserID) {
			return nil, categoryNotFound()
		}
		return nil, apperror.Forbidden("CATEGORY_FORBIDDEN", "category owner required")
	}
	return category, nil
}

func normalizeCategoryName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxCategoryNameLength {
		return "", apperror.InvalidInput("INVALID_NAME", "name must be 1-64 characters")
	}
	return name, nil
}

func validateCategoryDescription(description string) error {
	if len([]rune(description)) > maxCategoryDescriptionLength {
		return apperror.InvalidInput("INVALID_DESCRIPTION", "description must be at most 500 characters")
	}
	return nil
}

func categoryNotFound() error {
	return apperror.NotFound("CATEGORY_NOT_FOUND", "category not found")
}
