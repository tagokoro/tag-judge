package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	submissionLinkURLInvalidMessage = "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters"
	submissionLinkIDInvalidMessage  = "submission_link_id must be a positive integer"
)

// LibrarySubmissionLinkUsecase handles self-declared "related submission" URL
// links for a library version. A link is NOT a judge result:
// there is no Problem, adapter, Submission, Outbox, S3 object, Judge run,
// verdict, score, status, or case result. The URL is never fetched, parsed for
// content, or judged. The repository authorizes each read using the library's
// current visibility in the same SQL statement as the data fetch.
//
// The only dependency that can write is the link repository. Registration never
// reaches the existing Submission/Judge execution path.
type LibrarySubmissionLinkUsecase struct {
	versions        *LibraryVersionUsecase
	submissionLinks repository.LibrarySubmissionLinkRepository
}

func NewLibrarySubmissionLinkUsecase(
	versions *LibraryVersionUsecase,
	submissionLinks repository.LibrarySubmissionLinkRepository,
) *LibrarySubmissionLinkUsecase {
	return &LibrarySubmissionLinkUsecase{
		versions:        versions,
		submissionLinks: submissionLinks,
	}
}

// Create registers one self-declared submission URL link on a version. Any
// authenticated user may add a link to a public library; private libraries are
// hidden from everyone but the owner. Resolve enforces read visibility, and the
// repository re-checks the current visibility under a lock so a concurrent
// privatization cannot let a non-owner commit a link. The only write is the
// link row.
func (u *LibrarySubmissionLinkUsecase) Create(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int, rawURL string) (*domain.LibrarySubmissionLink, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if versionNumber <= 0 {
		return nil, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	}
	submissionURL, err := domain.ValidateSubmissionURL(rawURL)
	if err != nil {
		return nil, apperror.InvalidInput("INVALID_SUBMISSION_URL", submissionLinkURLInvalidMessage)
	}
	resolved, err := u.versions.Resolve(ctx, actor, namespaceSlug, librarySlug, versionNumber)
	if err != nil {
		return nil, err
	}
	return u.submissionLinks.Create(ctx, repository.CreateLibrarySubmissionLinkParams{
		ActorUserID:      actor.UserID,
		NamespaceID:      resolved.Library.NamespaceID,
		LibraryID:        resolved.Library.ID,
		LibraryVersionID: resolved.Version.ID,
		SubmissionURL:    submissionURL,
	})
}

// Get is the link detail. Authentication is NOT required: a public link is
// readable anonymously. A private link is indistinguishable from a missing one
// (404) for every non-owner, including admins, so existence is not leaked.
// The repository enforces this in the detail-fetch SQL, with no role bypass.
func (u *LibrarySubmissionLinkUsecase) Get(ctx context.Context, actor auth.Actor, submissionLinkID int64) (*domain.LibrarySubmissionLink, error) {
	if submissionLinkID <= 0 {
		return nil, apperror.InvalidInput("INVALID_LIBRARY_SUBMISSION_LINK_ID", submissionLinkIDInvalidMessage)
	}
	return u.submissionLinks.FindByID(ctx, submissionLinkID, actor.UserID)
}

// List returns the links for one version. Resolve resolves the namespace,
// library and version ids (and gives a steady-state 404 for a private library
// a non-owner cannot see), and the shared paging contract is applied. The
// actor's user id is threaded into ListByVersion, whose repository statement
// re-checks the library's CURRENT visibility in the SAME statement as the link
// fetch. That closes the Resolve->List TOCTOU window: if the owner privatizes
// the library between Resolve and the fetch, the fetch itself returns no
// now-private URL links to a non-owner (no admin bypass).
func (u *LibrarySubmissionLinkUsecase) List(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int, limit int, limitProvided bool, offset int) ([]domain.LibrarySubmissionLink, error) {
	limit, offset, err := validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	resolved, err := u.versions.Resolve(ctx, actor, namespaceSlug, librarySlug, versionNumber)
	if err != nil {
		return nil, err
	}
	return u.submissionLinks.ListByVersion(ctx, repository.ListLibrarySubmissionLinksParams{
		LibraryVersionID: resolved.Version.ID,
		Limit:            limit,
		Offset:           offset,
		ActorUserID:      actor.UserID,
	})
}
