package usecase

import (
	"context"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const maxNamespaceDisplayNameLength = 64

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

type NamespaceUsecase struct {
	namespaces repository.NamespaceRepository
}

func NewNamespaceUsecase(namespaces repository.NamespaceRepository) *NamespaceUsecase {
	return &NamespaceUsecase{namespaces: namespaces}
}

// Create gives the actor their single personal namespace. A user owns at most
// one namespace, so the repository enforces uniqueness and maps each unique
// constraint to its own conflict contract.
func (u *NamespaceUsecase) Create(ctx context.Context, actor auth.Actor, slug, displayName string) (*domain.Namespace, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(displayName)
	if !domain.ValidNamespaceSlug(slug) {
		return nil, apperror.InvalidInput("INVALID_SLUG", "slug must be 2-32 lowercase letters, digits, or single hyphens")
	}
	if name == "" || len([]rune(name)) > maxNamespaceDisplayNameLength {
		return nil, apperror.InvalidInput("INVALID_DISPLAY_NAME", "display_name must be 1-64 characters")
	}
	return u.namespaces.Create(ctx, repository.CreateNamespaceParams{
		OwnerUserID: actor.UserID,
		Slug:        slug,
		DisplayName: name,
	})
}

func (u *NamespaceUsecase) GetMine(ctx context.Context, actor auth.Actor) (*domain.Namespace, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	return u.namespaces.FindByOwner(ctx, actor.UserID)
}

func (u *NamespaceUsecase) GetBySlug(ctx context.Context, slug string) (*domain.Namespace, error) {
	return u.namespaces.FindBySlug(ctx, slug)
}
