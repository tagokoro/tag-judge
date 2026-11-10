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
}

// CreateLibrarySubmissionLinkParams carries a self-declared submission URL link.
// ActorUserID is the authenticated registrant (stored as created_by). The
// namespace/library/version ids are re-checked under a lock so a concurrent
// visibility change cannot let a non-owner commit a link.
type CreateLibrarySubmissionLinkParams struct {
	ActorUserID      string
	NamespaceID      string
	LibraryID        string
	LibraryVersionID string
	SubmissionURL    string
}

type ListLibrarySubmissionLinksParams struct {
	LibraryVersionID string
	Limit            int
	Offset           int
	ActorUserID      string
}

// LibrarySubmissionLinkRepository persists self-declared submission URL links.
// Create re-checks the library's current visibility under a row lock and the
// version's membership, and NEVER creates a Submission, Outbox record, S3
// object, or Judge run (no admission control). FindByID/ListByVersion enforce
// current library visibility in their data-fetch SQL statements.
type LibrarySubmissionLinkRepository interface {
	Create(ctx context.Context, params CreateLibrarySubmissionLinkParams) (*domain.LibrarySubmissionLink, error)
	FindByID(ctx context.Context, id int64, actorUserID string) (*domain.LibrarySubmissionLink, error)
	ListByVersion(ctx context.Context, params ListLibrarySubmissionLinksParams) ([]domain.LibrarySubmissionLink, error)
}

type NamespaceRepository interface {
	Create(ctx context.Context, params CreateNamespaceParams) (*domain.Namespace, error)
	FindByOwner(ctx context.Context, ownerUserID string) (*domain.Namespace, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Namespace, error)
}
