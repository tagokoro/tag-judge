package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	verificationAdapterInvalidMessage = "adapter_source must be 1-100000 bytes, non-blank, and must not contain NUL"
	verificationIDInvalidMessage      = "verification_id must be a positive integer"
)

type LibraryVerificationUsecase struct {
	versions      *LibraryVersionUsecase
	problems      repository.PublicProblemReader
	storage       repository.SourceCodeStorage
	verifications repository.LibraryVerificationRepository
	submissions   submissionDetailReader
}

// submissionDetailReader is the existing submission read path reused for
// verification case results; no new query object is introduced.
type submissionDetailReader interface {
	FindByID(ctx context.Context, submissionID int64) (*domain.Submission, error)
	ListCaseResults(ctx context.Context, submissionID int64) ([]domain.SubmissionCaseResult, error)
}

func NewLibraryVerificationUsecase(
	versions *LibraryVersionUsecase,
	problems repository.PublicProblemReader,
	storage repository.SourceCodeStorage,
	verifications repository.LibraryVerificationRepository,
	submissions submissionDetailReader,
) *LibraryVerificationUsecase {
	return &LibraryVerificationUsecase{
		versions:      versions,
		problems:      problems,
		storage:       storage,
		verifications: verifications,
		submissions:   submissions,
	}
}

// Create starts one verification. It combines the existing version source with
// adapter_source, stores the combined file on the existing storage path, and
// asks the repository to commit the submission rows in one DB transaction.
// Only the repository knows the final decision about visibility; this method
// never commits anything on a hidden library.
func (u *LibraryVerificationUsecase) Create(ctx context.Context, actor auth.Actor, namespaceSlug, librarySlug string, versionNumber, problemID int64, adapterSource string) (*domain.LibraryVerification, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if problemID <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	if versionNumber <= 0 {
		return nil, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	}
	if err := librarycombine.ValidateSource(adapterSource); err != nil {
		return nil, apperror.InvalidInput("INVALID_ADAPTER_SOURCE", verificationAdapterInvalidMessage)
	}

	problem, err := u.problems.FindPublicByID(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if problem.CurrentTestCasesKey == "" {
		return nil, apperror.Conflict("PROBLEM_HAS_NO_TEST_CASES", "problem has no test cases")
	}

	resolved, err := u.versions.Resolve(ctx, actor, namespaceSlug, librarySlug, int(versionNumber))
	if err != nil {
		return nil, err
	}

	combined, err := librarycombine.Combine(resolved.Version.SourceCode, adapterSource)
	if err != nil {
		return nil, apperror.InvalidInput("INVALID_ADAPTER_SOURCE", verificationAdapterInvalidMessage)
	}
	sourceCodeKey, err := newSourceCodeKey(actor.UserID, problem.ID, domain.SubmissionLanguagePython)
	if err != nil {
		return nil, err
	}
	if err := u.storage.PutSourceCode(ctx, repository.PutSourceCodeParams{
		Key:        sourceCodeKey,
		Language:   domain.ExecutionLanguagePython,
		SourceCode: combined,
	}); err != nil {
		return nil, err
	}

	verification, _, err := u.verifications.CreateWithSubmission(ctx, repository.CreateLibraryVerificationParams{
		ActorUserID:      actor.UserID,
		NamespaceID:      resolved.Library.NamespaceID,
		LibraryID:        resolved.Library.ID,
		LibraryVersionID: resolved.Version.ID,
		ProblemID:        problem.ID,
		Language:         domain.SubmissionLanguagePython,
		SourceCodeKey:    sourceCodeKey,
		TestCasesKey:     problem.CurrentTestCasesKey,
		TimeLimitMS:      problem.TimeLimitMS,
		AdapterSource:    adapterSource,
	})
	if err == nil {
		verification.NamespaceSlug = resolved.Library.NamespaceSlug
		verification.LibrarySlug = resolved.Library.Slug
		verification.VersionNumber = resolved.Version.VersionNumber
	}
	return verification, err
}

// Get is the verification detail. adapter_source is visible only to the
// starter; admins read the same detail they may read for submissions.
func (u *LibraryVerificationUsecase) Get(ctx context.Context, actor auth.Actor, verificationID int64) (*domain.LibraryVerification, []domain.SubmissionCaseResult, error) {
	if err := validateActor(actor); err != nil {
		return nil, nil, err
	}
	if verificationID <= 0 {
		return nil, nil, apperror.InvalidInput("INVALID_LIBRARY_VERIFICATION_ID", verificationIDInvalidMessage)
	}
	verification, err := u.verifications.FindByID(ctx, verificationID)
	if err != nil {
		return nil, nil, err
	}
	if actor.UserID != verification.UserID && actor.Role != domain.UserRoleAdmin {
		return nil, nil, apperror.Forbidden("LIBRARY_VERIFICATION_FORBIDDEN", "library verification is not accessible")
	}
	results, err := u.submissions.ListCaseResults(ctx, verification.SubmissionID)
	if err != nil {
		return nil, nil, err
	}
	if results == nil {
		results = []domain.SubmissionCaseResult{}
	}
	return verification, results, nil
}

// List returns only the caller's own verifications for every role, admins
// included; the authorization table has no admin bypass here.
func (u *LibraryVerificationUsecase) List(ctx context.Context, actor auth.Actor, limit int, limitProvided bool, offset int) ([]domain.LibraryVerification, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	limit, offset, err := validatePaging(limit, limitProvided, offset)
	if err != nil {
		return nil, err
	}
	return u.verifications.ListByUserID(ctx, repository.ListLibraryVerificationsParams{
		UserID: actor.UserID,
		Limit:  limit,
		Offset: offset,
	})
}
