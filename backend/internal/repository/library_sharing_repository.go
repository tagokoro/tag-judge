package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateNamespaceParams struct {
	OwnerUserID string
	Slug        string
	DisplayName string
}

type CreateLibraryParams struct {
	NamespaceID string
	Slug        string
	Name        string
	Description string
	Visibility  domain.LibraryVisibility
}

// UpdateLibraryParams uses nil to mark fields the PATCH does not touch.
type UpdateLibraryParams struct {
	ID          string
	Name        *string
	Description *string
	Visibility  *string
}

type LibraryRepository interface {
	Create(ctx context.Context, params CreateLibraryParams) (*domain.Library, error)
	Update(ctx context.Context, params UpdateLibraryParams) (*domain.Library, error)
	FindBySlug(ctx context.Context, namespaceID, slug string) (*domain.Library, error)
	ListPublic(ctx context.Context, namespaceID string, limit, offset int) ([]domain.Library, error)
	ListAll(ctx context.Context, namespaceID string, limit, offset int) ([]domain.Library, error)
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

type NamespaceRepository interface {
	Create(ctx context.Context, params CreateNamespaceParams) (*domain.Namespace, error)
	FindByOwner(ctx context.Context, ownerUserID string) (*domain.Namespace, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Namespace, error)
}
