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
	LibraryID     string
	SourceCode    string
	ContentSHA256 string
	ByteSize      int
	CreatedBy     string
}

type ListLibraryVersionsParams struct {
	LibraryID string
	Limit     int
	Offset    int
	// ActorUserID is the reader's user id (empty for anonymous). The repository
	// uses it in the SAME SQL statement as the version fetch to gate rows on the
	// library's CURRENT visibility, so a concurrent privatization cannot expose
	// source that was already private when the rows were read.
	ActorUserID string
}

type LibraryVersionRepository interface {
	Create(ctx context.Context, params CreateLibraryVersionParams) (*domain.LibraryVersion, error)
	// FindByNumber re-checks the library's CURRENT visibility with actorUserID in
	// the same statement that returns the version body; a private library is
	// visible only to its owner (404 for everyone else, admins included).
	FindByNumber(ctx context.Context, libraryID string, versionNumber int, actorUserID string) (*domain.LibraryVersion, error)
	ListSummaries(ctx context.Context, params ListLibraryVersionsParams) ([]domain.LibraryVersion, error)
}

// CreateLibraryVerificationParams carries a self-declared submission URL link.
// ActorUserID is the authenticated registrant (stored as created_by). The
// namespace/library/version ids are re-checked under a lock so a concurrent
// visibility change cannot let a non-owner commit a link.
type CreateLibraryVerificationParams struct {
	ActorUserID      string
	NamespaceID      string
	LibraryID        string
	LibraryVersionID string
	SubmissionURL    string
}

type ListLibraryVerificationsParams struct {
	LibraryVersionID string
	Limit            int
	Offset           int
	// ActorUserID is the reader's user id (empty for anonymous). ListByVersion
	// gates every row on the library's CURRENT visibility with this id in the
	// SAME SQL statement, so a public->private flip between a prior Resolve and
	// this fetch cannot expose now-private URL links.
	ActorUserID string
}

// LibraryVerificationRepository persists self-declared submission URL links.
// Create re-checks the library's current visibility under a row lock and the
// version's membership, and NEVER creates a Submission, Outbox record, S3
// object, or Judge run (no admission control). FindByID joins the current
// library/namespace in a single statement so the caller authorizes the detail
// read against a visibility snapshot consistent with the returned data.
// ListByVersion enforces the CURRENT visibility (owner-only for private) in the
// same statement as the fetch via an actor-id SQL predicate.
type LibraryVerificationRepository interface {
	Create(ctx context.Context, params CreateLibraryVerificationParams) (*domain.LibraryVerification, error)
	FindByID(ctx context.Context, id int64) (*domain.LibraryVerification, error)
	ListByVersion(ctx context.Context, params ListLibraryVerificationsParams) ([]domain.LibraryVerification, error)
}

type NamespaceRepository interface {
	Create(ctx context.Context, params CreateNamespaceParams) (*domain.Namespace, error)
	FindByOwner(ctx context.Context, ownerUserID string) (*domain.Namespace, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Namespace, error)
}
