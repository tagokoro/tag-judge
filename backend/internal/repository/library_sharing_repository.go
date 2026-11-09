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
}

type LibraryVersionRepository interface {
	Create(ctx context.Context, params CreateLibraryVersionParams) (*domain.LibraryVersion, error)
	FindByNumber(ctx context.Context, libraryID string, versionNumber int) (*domain.LibraryVersion, error)
	ListSummaries(ctx context.Context, params ListLibraryVersionsParams) ([]domain.LibraryVersion, error)
}

type CreateLibraryVerificationParams struct {
	ActorUserID      string
	NamespaceID      string
	LibraryID        string
	LibraryVersionID string
	ProblemID        int64
	Language         domain.SubmissionLanguage
	SourceCodeKey    string
	TestCasesKey     string
	TimeLimitMS      int
	AdapterSource    string
}

type ListLibraryVerificationsParams struct {
	UserID string
	Limit  int
	Offset int
}

type LibraryVerificationRepository interface {
	CreateWithSubmission(ctx context.Context, params CreateLibraryVerificationParams) (*domain.LibraryVerification, *domain.Submission, error)
	FindByID(ctx context.Context, id int64) (*domain.LibraryVerification, error)
	ListByUserID(ctx context.Context, params ListLibraryVerificationsParams) ([]domain.LibraryVerification, error)
}

type NamespaceRepository interface {
	Create(ctx context.Context, params CreateNamespaceParams) (*domain.Namespace, error)
	FindByOwner(ctx context.Context, ownerUserID string) (*domain.Namespace, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Namespace, error)
}
