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
	namespaces repository.NamespaceRepository
	libraries  repository.LibraryRepository
}

func NewLibraryUsecase(namespaces repository.NamespaceRepository, libraries repository.LibraryRepository) *LibraryUsecase {
	return &LibraryUsecase{namespaces: namespaces, libraries: libraries}
}

type CreateLibraryInput struct {
	Slug        string
	Name        string
	Description string
	Visibility  string
}

type UpdateLibraryInput struct {
	Name        *string
	Description *string
	Visibility  *string
}

// libraryContext resolves the namespace behind a namespace slug so every
// library route shares one lookup and one not-found contract.
type libraryContext struct {
	namespace domain.Namespace
}

func (u *LibraryUsecase) resolveNamespace(ctx context.Context, namespaceSlug string) (*libraryContext, error) {
	namespace, err := u.namespaces.FindBySlug(ctx, namespaceSlug)
	if err != nil {
		return nil, err
	}
	return &libraryContext{namespace: *namespace}, nil
}

// readable reports whether actor may see library. private libraries hide from
// everyone except the namespace owner, including admins.
func readable(library domain.Library, ownerUserID string, actor auth.Actor) bool {
	return library.Visibility != domain.LibraryVisibilityPrivate || actor.UserID == ownerUserID
}

func (u *LibraryUsecase) Create(ctx context.Context, actor auth.Actor, namespaceSlug string, input CreateLibraryInput) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	resolved, err := u.resolveNamespace(ctx, namespaceSlug)
	if err != nil {
		return nil, err
	}
	if resolved.namespace.OwnerUserID != actor.UserID {
		return nil, apperror.Forbidden("LIBRARY_FORBIDDEN", "library owner required")
	}

	name := strings.TrimSpace(input.Name)
	if !domain.ValidLibrarySlug(input.Slug) {
		return nil, apperror.InvalidInput("INVALID_SLUG", "slug must be 2-48 lowercase letters, digits, or single hyphens")
	}
	if name == "" || len([]rune(name)) > maxLibraryNameLength {
		return nil, apperror.InvalidInput("INVALID_NAME", "name must be 1-80 characters")
	}
	if len([]rune(input.Description)) > maxLibraryDescriptionLength {
		return nil, apperror.InvalidInput("INVALID_DESCRIPTION", "description must be at most 2000 characters")
	}
	visibility := domain.LibraryVisibility(input.Visibility)
	if !visibility.Valid() {
		return nil, apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public or private")
	}

	library, err := u.libraries.Create(ctx, repository.CreateLibraryParams{
		NamespaceID: resolved.namespace.ID,
		Slug:        input.Slug,
		Name:        name,
		Description: input.Description,
		Visibility:  visibility,
	})
	if err != nil {
		return nil, err
	}
	library.NamespaceSlug = resolved.namespace.Slug
	return library, nil
}

func (u *LibraryUsecase) Get(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string) (*domain.Library, error) {
	resolved, err := u.resolveNamespace(ctx, namespaceSlug)
	if err != nil {
		return nil, err
	}
	library, err := u.libraries.FindBySlug(ctx, resolved.namespace.ID, librarySlug)
	if err != nil {
		return nil, err
	}
	if !readable(*library, resolved.namespace.OwnerUserID, actor) {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	library.NamespaceSlug = resolved.namespace.Slug
	return library, nil
}

// AuthorizeOwner is the write gate for every owner-only operation. A public
// library read by a stranger becomes Forbidden; a private library hidden from
// a stranger stays indistinguishable from a missing one.
func (u *LibraryUsecase) AuthorizeOwner(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	resolved, err := u.resolveNamespace(ctx, namespaceSlug)
	if err != nil {
		return nil, err
	}
	library, err := u.libraries.FindBySlug(ctx, resolved.namespace.ID, librarySlug)
	if err != nil {
		return nil, err
	}
	if resolved.namespace.OwnerUserID != actor.UserID {
		if library.Visibility == domain.LibraryVisibilityPrivate {
			return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
		}
		return nil, apperror.Forbidden("LIBRARY_FORBIDDEN", "library owner required")
	}
	library.NamespaceSlug = resolved.namespace.Slug
	return library, nil
}

func (u *LibraryUsecase) List(ctx context.Context, actor auth.Actor, namespaceSlug string, limit int, limitProvided bool, offset int) ([]domain.Library, error) {
	resolved, err := u.resolveNamespace(ctx, namespaceSlug)
	if err != nil {
		return nil, err
	}
	limit, offset, err = validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	var libraries []domain.Library
	if actor.UserID == resolved.namespace.OwnerUserID && actor.UserID != "" {
		libraries, err = u.libraries.ListAll(ctx, resolved.namespace.ID, limit, offset)
	} else {
		libraries, err = u.libraries.ListPublic(ctx, resolved.namespace.ID, limit, offset)
	}
	for i := range libraries {
		libraries[i].NamespaceSlug = resolved.namespace.Slug
	}
	return libraries, err
}

func (u *LibraryUsecase) Update(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, input UpdateLibraryInput) (*domain.Library, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if input.Name == nil && input.Description == nil && input.Visibility == nil {
		return nil, apperror.InvalidInput("EMPTY_UPDATE", "at least one field is required")
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len([]rune(name)) > maxLibraryNameLength {
			return nil, apperror.InvalidInput("INVALID_NAME", "name must be 1-80 characters")
		}
		input.Name = &name
	}
	if input.Visibility != nil && !domain.LibraryVisibility(*input.Visibility).Valid() {
		return nil, apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public or private")
	}
	if input.Description != nil && len([]rune(*input.Description)) > maxLibraryDescriptionLength {
		return nil, apperror.InvalidInput("INVALID_DESCRIPTION", "description must be at most 2000 characters")
	}

	library, err := u.AuthorizeOwner(ctx, actor, namespaceSlug, librarySlug)
	if err != nil {
		return nil, err
	}

	updated, err := u.libraries.Update(ctx, repository.UpdateLibraryParams{
		ID:          library.ID,
		Name:        input.Name,
		Description: input.Description,
		Visibility:  input.Visibility,
	})
	if err != nil {
		return nil, err
	}
	updated.NamespaceSlug = library.NamespaceSlug
	return updated, nil
}
