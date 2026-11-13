package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

// ListCategoriesParams lists one owner's categories. IncludeHidden also lists
// categories without a public library, which only their owner may see.
type ListCategoriesParams struct {
	OwnerUserID   string
	IncludeHidden bool
	Limit         int
	Offset        int
}

// UpdateCategoryParams uses nil to mark fields the PATCH does not touch. ID is
// the internal category id.
type UpdateCategoryParams struct {
	ID          string
	Name        *string
	Description *string
}

// CreateLibraryParams files the library under the owner's category named
// CategoryName, creating that category when the owner has none by that name.
type CreateLibraryParams struct {
	OwnerUserID  string
	CategoryName string
	Name         string
	Description  string
	Visibility   domain.LibraryVisibility
}

// UpdateLibraryParams uses nil to mark fields the PATCH does not touch.
// CategoryName moves the library to the owner's category by that name,
// creating it when needed; a category left empty by the move is removed.
// OwnerUserID is the library's owner, whom the caller has authorized.
type UpdateLibraryParams struct {
	ID           string
	OwnerUserID  string
	Name         *string
	Description  *string
	Visibility   *string
	CategoryName *string
}

// ListLibrariesParams lists one category's libraries (internal category id).
// IncludePrivate also lists private ones, for the owner.
type ListLibrariesParams struct {
	CategoryID     string
	IncludePrivate bool
	Limit          int
	Offset         int
}

// Every method returns libraries with their category and owner filled in.
// Create and Update enforce that the owner has a handle and stays within
// domain.MaxCategoriesPerUser when they add a category.
type LibraryRepository interface {
	Create(ctx context.Context, params CreateLibraryParams) (*domain.Library, error)
	Update(ctx context.Context, params UpdateLibraryParams) (*domain.Library, error)
	FindByPublicID(ctx context.Context, publicID string) (*domain.Library, error)
	ListByCategory(ctx context.Context, params ListLibrariesParams) ([]domain.Library, error)
	// ListAllPublic lists public libraries across every category.
	ListAllPublic(ctx context.Context, limit, offset int) ([]domain.Library, error)
}

type CreateLibraryVersionParams struct {
	LibraryID  string
	SourceCode string
	CreatedBy  string
}

type ListLibraryVersionsParams struct {
	LibraryID   string
	Limit       int
	Offset      int
	ActorUserID string
}

type LibraryVersionRepository interface {
	Create(ctx context.Context, params CreateLibraryVersionParams) (*domain.LibraryVersion, error)
	FindByNumber(ctx context.Context, libraryID string, versionNumber int, actorUserID string) (*domain.LibraryVersion, error)
	ListSummaries(ctx context.Context, params ListLibraryVersionsParams) ([]domain.LibraryVersion, error)
	SetSubmissionURL(ctx context.Context, params SetLibraryVersionSubmissionURLParams) (*domain.LibraryVersion, error)
}

// SetLibraryVersionSubmissionURLParams sets (non-nil) or clears (nil) the
// self-declared submission URL of one version. Only the library owner may
// write it; the ownership check runs in the same SQL statement as the update.
type SetLibraryVersionSubmissionURLParams struct {
	LibraryID     string
	VersionNumber int
	ActorUserID   string
	SubmissionURL *string
}

// CategoryRepository reads and edits categories. They are created and removed
// only through LibraryRepository, together with the libraries in them.
type CategoryRepository interface {
	// FindByPublicID finds a category whoever may see it; callers apply
	// Category.VisibleTo.
	FindByPublicID(ctx context.Context, publicID string) (*domain.Category, error)
	ListByOwner(ctx context.Context, params ListCategoriesParams) ([]domain.Category, error)
	Update(ctx context.Context, params UpdateCategoryParams) (*domain.Category, error)
}
