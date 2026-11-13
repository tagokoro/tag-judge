package usecase

import (
	"context"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	maxLibraryNameLength        = 80
	maxLibraryDescriptionLength = 2000
)

type LibraryUsecase struct {
	categories repository.CategoryRepository
	libraries  repository.LibraryRepository
}

func NewLibraryUsecase(categories repository.CategoryRepository, libraries repository.LibraryRepository) *LibraryUsecase {
	return &LibraryUsecase{categories: categories, libraries: libraries}
}

// CreateLibraryInput names the category to file the library under; the
// owner's category by that name is used or created.
type CreateLibraryInput struct {
	Name        string
	Description string
	Visibility  string
	Category    string
}

// UpdateLibraryInput uses nil for fields the PATCH does not touch. Category
// moves the library to the owner's category by that name, creating it when
// needed.
type UpdateLibraryInput struct {
	Name        *string
	Description *string
	Visibility  *string
	Category    *string
}

// findCategory resolves a category public id. A malformed id cannot exist,
// so it shares the not-found contract with a missing category.
func (u *LibraryUsecase) findCategory(ctx context.Context, publicID string) (*domain.Category, error) {
	if !domain.ValidPublicID(publicID) {
		return nil, categoryNotFound()
	}
	return u.categories.FindByPublicID(ctx, publicID)
}

func (u *LibraryUsecase) findLibrary(ctx context.Context, publicID string) (*domain.Library, error) {
	if !domain.ValidPublicID(publicID) {
		return nil, libraryNotFound()
	}
	return u.libraries.FindByPublicID(ctx, publicID)
}

// Create files a new library under the actor's category named in the input,
// creating the category when the actor has none by that name.
func (u *LibraryUsecase) Create(ctx context.Context, actor auth.Actor, input CreateLibraryInput) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	name, err := normalizeLibraryName(input.Name)
	if err != nil {
		return nil, err
	}
	category, err := normalizeLibraryCategory(input.Category)
	if err != nil {
		return nil, err
	}
	if len([]rune(input.Description)) > maxLibraryDescriptionLength {
		return nil, apperror.InvalidInput("INVALID_DESCRIPTION", "description must be at most 2000 characters")
	}
	visibility := domain.LibraryVisibility(input.Visibility)
	if !visibility.Valid() {
		return nil, apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public or private")
	}

	return u.libraries.Create(ctx, repository.CreateLibraryParams{
		OwnerUserID:  actor.UserID,
		CategoryName: category,
		Name:         name,
		Description:  input.Description,
		Visibility:   visibility,
	})
}

func (u *LibraryUsecase) Get(ctx context.Context, actor auth.Actor, libraryID string) (*domain.Library, error) {
	library, err := u.findLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	if !library.ReadableBy(actor.UserID) {
		return nil, libraryNotFound()
	}
	return library, nil
}

// AuthorizeOwner is the write gate for every owner-only operation. A public
// library read by a stranger becomes Forbidden; a private library hidden from
// a stranger stays indistinguishable from a missing one.
func (u *LibraryUsecase) AuthorizeOwner(ctx context.Context, actor auth.Actor, libraryID string) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	library, err := u.findLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	if library.OwnerUserID != actor.UserID {
		if !library.ReadableBy(actor.UserID) {
			return nil, libraryNotFound()
		}
		return nil, apperror.Forbidden("LIBRARY_FORBIDDEN", "library owner required")
	}
	return library, nil
}

// List lists a category's libraries the actor may see. A category hidden
// from the actor is a 404, like on its own route.
func (u *LibraryUsecase) List(ctx context.Context, actor auth.Actor, categoryID string, limit int, limitProvided bool, offset int) ([]domain.Library, error) {
	category, err := u.findCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if !category.VisibleTo(actor.UserID) {
		return nil, categoryNotFound()
	}
	limit, offset, err = validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	return u.libraries.ListByCategory(ctx, repository.ListLibrariesParams{
		CategoryID:     category.ID,
		IncludePrivate: actor.UserID != "" && actor.UserID == category.OwnerUserID,
		Limit:          limit,
		Offset:         offset,
	})
}

// ListAllPublic lists public libraries across every category. Anyone may call
// it; private libraries are never included.
func (u *LibraryUsecase) ListAllPublic(ctx context.Context, limit int, limitProvided bool, offset int) ([]domain.Library, error) {
	limit, offset, err := validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	return u.libraries.ListAllPublic(ctx, limit, offset)
}

// Update changes the library and, with Category, moves it to the owner's
// category by that name (created when needed; the category it leaves is
// removed once empty).
func (u *LibraryUsecase) Update(ctx context.Context, actor auth.Actor, libraryID string, input UpdateLibraryInput) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if input.Name == nil && input.Description == nil && input.Visibility == nil && input.Category == nil {
		return nil, apperror.InvalidInput("EMPTY_UPDATE", "at least one field is required")
	}
	if input.Name != nil {
		name, err := normalizeLibraryName(*input.Name)
		if err != nil {
			return nil, err
		}
		input.Name = &name
	}
	if input.Category != nil {
		category, err := normalizeLibraryCategory(*input.Category)
		if err != nil {
			return nil, err
		}
		input.Category = &category
	}
	if input.Visibility != nil && !domain.LibraryVisibility(*input.Visibility).Valid() {
		return nil, apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public or private")
	}
	if input.Description != nil && len([]rune(*input.Description)) > maxLibraryDescriptionLength {
		return nil, apperror.InvalidInput("INVALID_DESCRIPTION", "description must be at most 2000 characters")
	}

	library, err := u.AuthorizeOwner(ctx, actor, libraryID)
	if err != nil {
		return nil, err
	}
	return u.libraries.Update(ctx, repository.UpdateLibraryParams{
		ID:           library.ID,
		OwnerUserID:  library.OwnerUserID,
		Name:         input.Name,
		Description:  input.Description,
		Visibility:   input.Visibility,
		CategoryName: input.Category,
	})
}

func normalizeLibraryName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxLibraryNameLength {
		return "", apperror.InvalidInput("INVALID_NAME", "name must be 1-80 characters")
	}
	return name, nil
}

// normalizeLibraryCategory checks the category name a library is filed under
// with the category name rule, reporting it against the category field.
func normalizeLibraryCategory(raw string) (string, error) {
	name, err := normalizeCategoryName(raw)
	if err != nil {
		return "", apperror.InvalidInput("INVALID_CATEGORY", "category must be 1-64 characters")
	}
	return name, nil
}

func libraryNotFound() error {
	return apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
}
