package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	verificationURLInvalidMessage = "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters"
	verificationIDInvalidMessage  = "verification_id must be a positive integer"
)

// LibraryVerificationUsecase handles self-declared "related submission" URL
// links for a library version. A link is NOT a verification or a judge result:
// there is no Problem, adapter, Submission, Outbox, S3 object, Judge run,
// verdict, score, status, or case result. The URL is never fetched, parsed for
// content, or judged. Read authorization uses the library's CURRENT visibility,
// which the repository joins onto every link at read time.
//
// The only dependency that can write is the link repository. Registration never
// reaches the existing Submission/Judge execution path.
type LibraryVerificationUsecase struct {
	versions      *LibraryVersionUsecase
	verifications repository.LibraryVerificationRepository
}

func NewLibraryVerificationUsecase(
	versions *LibraryVersionUsecase,
	verifications repository.LibraryVerificationRepository,
) *LibraryVerificationUsecase {
	return &LibraryVerificationUsecase{
		versions:      versions,
		verifications: verifications,
	}
}

// Create registers one self-declared submission URL link on a version. Any
// authenticated user may add a link to a public library; private libraries are
// hidden from everyone but the owner. Resolve enforces read visibility, and the
// repository re-checks the current visibility under a lock so a concurrent
// privatization cannot let a non-owner commit a link. The only write is the
// link row.
func (u *LibraryVerificationUsecase) Create(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int, rawURL string) (*domain.LibraryVerification, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if versionNumber <= 0 {
		return nil, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	}
	submissionURL, err := domain.ValidateSubmissionURL(rawURL)
	if err != nil {
		return nil, apperror.InvalidInput("INVALID_SUBMISSION_URL", verificationURLInvalidMessage)
	}
	resolved, err := u.versions.Resolve(ctx, actor, namespaceSlug, librarySlug, versionNumber)
	if err != nil {
		return nil, err
	}
	return u.verifications.Create(ctx, repository.CreateLibraryVerificationParams{
		ActorUserID:      actor.UserID,
		NamespaceID:      resolved.Library.NamespaceID,
		LibraryID:        resolved.Library.ID,
		LibraryVersionID: resolved.Version.ID,
		SubmissionURL:    submissionURL,
	})
}

// verificationReadable reports whether the link's CURRENT library visibility
// allows actor to read it. Public links are viewable by anyone (anonymous
// included); private links only by the library owner, with no admin bypass.
func verificationReadable(link domain.LibraryVerification, actor auth.Actor) bool {
	return link.Visibility != domain.LibraryVisibilityPrivate || actor.UserID == link.OwnerUserID
}

// Get is the link detail. Authentication is NOT required: a public link is
// readable anonymously. A private link is indistinguishable from a missing one
// (404) for every non-owner, including admins, so existence is not leaked.
//
// Atomicity: FindByID is a SINGLE statement that returns the link data joined
// to the library's CURRENT visibility and owner, so verificationReadable is
// applied to the same MVCC snapshot as the returned data (there is no
// check-then-fetch window here). No role is consulted: only the owner user id.
func (u *LibraryVerificationUsecase) Get(ctx context.Context, actor auth.Actor, verificationID int64) (*domain.LibraryVerification, error) {
	if verificationID <= 0 {
		return nil, apperror.InvalidInput("INVALID_LIBRARY_VERIFICATION_ID", verificationIDInvalidMessage)
	}
	link, err := u.verifications.FindByID(ctx, verificationID)
	if err != nil {
		return nil, err
	}
	if !verificationReadable(*link, actor) {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	return link, nil
}

// List returns the links for one version. Resolve resolves the namespace,
// library and version ids (and gives a steady-state 404 for a private library
// a non-owner cannot see), and the shared paging contract is applied. The
// actor's user id is threaded into ListByVersion, whose repository statement
// re-checks the library's CURRENT visibility in the SAME statement as the link
// fetch. That closes the Resolve->List TOCTOU window: if the owner privatizes
// the library between Resolve and the fetch, the fetch itself returns no
// now-private URL links to a non-owner (no admin bypass).
func (u *LibraryVerificationUsecase) List(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber int, limit int, limitProvided bool, offset int) ([]domain.LibraryVerification, error) {
	limit, offset, err := validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	resolved, err := u.versions.Resolve(ctx, actor, namespaceSlug, librarySlug, versionNumber)
	if err != nil {
		return nil, err
	}
	return u.verifications.ListByVersion(ctx, repository.ListLibraryVerificationsParams{
		LibraryVersionID: resolved.Version.ID,
		Limit:            limit,
		Offset:           offset,
		ActorUserID:      actor.UserID,
	})
}
