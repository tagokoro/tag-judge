package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type LibraryVersionUsecase struct {
	libraries *LibraryUsecase
	versions  repository.LibraryVersionRepository
}

func NewLibraryVersionUsecase(libraries *LibraryUsecase, versions repository.LibraryVersionRepository) *LibraryVersionUsecase {
	return &LibraryVersionUsecase{libraries: libraries, versions: versions}
}

// Create appends one immutable version to a library the actor owns. Version
// numbers come from the database; this method never counts existing versions.
func (u *LibraryVersionUsecase) Create(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug, sourceCode string) (*domain.LibraryVersion, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	library, err := u.libraries.AuthorizeOwner(ctx, actor, namespaceSlug, librarySlug)
	if err != nil {
		return nil, err
	}
	if err := librarycombine.ValidateSource(sourceCode); err != nil {
		return nil, apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code must be 1-100000 bytes, non-blank, and must not contain NUL")
	}
	return u.versions.Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:     library.ID,
		SourceCode:    sourceCode,
		ContentSHA256: librarycombine.ContentSHA256(sourceCode),
		ByteSize:      len([]byte(sourceCode)),
		CreatedBy:     actor.UserID,
	})
}

func (u *LibraryVersionUsecase) Get(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int) (*domain.LibraryVersion, error) {
	library, err := u.libraries.Get(ctx, actor, namespaceSlug, librarySlug)
	if err != nil {
		return nil, err
	}
	return u.versions.FindByNumber(ctx, library.ID, versionNumber)
}

func (u *LibraryVersionUsecase) List(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, limit int, limitProvided bool, offset int) ([]domain.LibraryVersion, error) {
	library, err := u.libraries.Get(ctx, actor, namespaceSlug, librarySlug)
	if err != nil {
		return nil, err
	}
	limit, offset, err = validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	return u.versions.ListSummaries(ctx, repository.ListLibraryVersionsParams{
		LibraryID: library.ID,
		Limit:     limit,
		Offset:    offset,
	})
}

// ResolvedLibraryVersion is the pair returned by Resolve: the readable library
// plus one of its versions. Verify builds on exactly this authorization.
type ResolvedLibraryVersion struct {
	Library domain.Library
	Version domain.LibraryVersion
}

// Resolve exposes library+version with the same visibility rules as Get. The
// version row itself is fetched by number and checked to belong to the
// library before any side effect runs.
func (u *LibraryVersionUsecase) Resolve(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int) (*ResolvedLibraryVersion, error) {
	if versionNumber <= 0 {
		return nil, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	}
	resolved, err := u.libraries.Get(ctx, actor, namespaceSlug, librarySlug)
	if err != nil {
		return nil, err
	}
	version, err := u.versions.FindByNumber(ctx, resolved.ID, versionNumber)
	if err != nil {
		return nil, err
	}
	return &ResolvedLibraryVersion{Library: *resolved, Version: *version}, nil
}
