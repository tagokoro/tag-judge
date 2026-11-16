package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	defaultSubmissionLimit = 20
	maxSubmissionLimit     = 100
)

type SubmissionUsecase struct {
	submissions          repository.SubmissionRepository
	problems             repository.ProblemReader
	storage              repository.SubmissionSourceStorage
	admission            repository.AdmissionControl
	idempotencyKeyMaxLen int
}

func NewSubmissionUsecase(
	submissions repository.SubmissionRepository,
	problems repository.ProblemReader,
	storage repository.SubmissionSourceStorage,
	admission repository.AdmissionControl,
	idempotencyKeyMaxLen int,
) *SubmissionUsecase {
	return &SubmissionUsecase{
		submissions:          submissions,
		problems:             problems,
		storage:              storage,
		admission:            admission,
		idempotencyKeyMaxLen: idempotencyKeyMaxLen,
	}
}

type CreateSubmissionInput struct {
	Actor          auth.Actor
	ProblemID      int64
	Language       domain.SubmissionLanguage
	SourceCode     string
	IdempotencyKey string
}

func (u *SubmissionUsecase) Create(ctx context.Context, input CreateSubmissionInput) (*domain.Submission, error) {
	if err := validateActor(input.Actor); err != nil {
		return nil, err
	}
	if input.ProblemID <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	if !input.Language.Valid() {
		return nil, apperror.InvalidInput("INVALID_LANGUAGE", "language must be python")
	}
	if err := validateSourceCode(input.SourceCode); err != nil {
		return nil, err
	}
	idempotency, err := buildSubmissionIdempotency(input.Actor.UserID, u.idempotencyKeyMaxLen, input)
	if err != nil {
		return nil, err
	}

	// Problem access policy lives here (problem_access.go), not in the handler
	// or the persistence adapter.
	problem, err := u.problems.FindByID(ctx, input.ProblemID)
	if err != nil {
		return nil, err
	}
	if !canSubmitProblem(input.Actor, *problem) {
		return nil, problemNotFound()
	}
	if strings.TrimSpace(problem.CurrentTestCasesKey) == "" {
		return nil, apperror.Conflict("PROBLEM_HAS_NO_TEST_CASES", "problem has no test cases")
	}

	sourceCodeKey, err := newSourceCodeKey(input.Actor.UserID, input.ProblemID, input.Language)
	if err != nil {
		return nil, err
	}
	if err := u.storage.PutSourceCode(ctx, repository.PutSourceCodeParams{
		Key:        sourceCodeKey,
		Language:   input.Language,
		SourceCode: input.SourceCode,
	}); err != nil {
		return nil, err
	}

	submission, err := u.admission.CreateSubmission(ctx, repository.CreateSubmissionWithOutboxParams{
		CreateSubmissionParams: repository.CreateSubmissionParams{
			UserID:        input.Actor.UserID,
			ProblemID:     input.ProblemID,
			Language:      input.Language,
			SourceCodeKey: sourceCodeKey,
			TestCasesKey:  problem.CurrentTestCasesKey,
			Status:        domain.SubmissionStatusPending,
			Score:         0,
			// A problem only ever becomes public, so a submission made while it
			// was a draft safely stays private.
			IsPublic: problem.Visibility == domain.ProblemVisibilityPublic,
		},
		TimeLimitMS: problem.TimeLimitMS,
	}, idempotency)
	if err != nil {
		return nil, err
	}

	return submission, nil
}

func buildSubmissionIdempotency(userID string, maxLen int, input CreateSubmissionInput) (*repository.IdempotencyKeyInfo, error) {
	fingerprint, err := submissionFingerprint(input)
	if err != nil {
		return nil, err
	}
	return newIdempotencyKey(userID, repository.IdempotencyNamespaceSubmission, input.IdempotencyKey, fingerprint, maxLen)
}

func submissionFingerprint(input CreateSubmissionInput) (string, error) {
	return canonicalFingerprint(struct {
		ProblemID  int64  `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
	}{
		ProblemID:  input.ProblemID,
		Language:   string(input.Language),
		SourceCode: input.SourceCode,
	})
}

// GetSubmissionInput takes the zero Actor for an anonymous reader.
type GetSubmissionInput struct {
	Actor        auth.Actor
	SubmissionID int64
}

func (u *SubmissionUsecase) Get(ctx context.Context, input GetSubmissionInput) (*domain.SubmissionDetail, error) {
	if input.SubmissionID <= 0 {
		return nil, apperror.InvalidInput("INVALID_SUBMISSION_ID", "submission_id must be a positive integer")
	}

	submission, err := u.submissions.FindByID(ctx, input.SubmissionID)
	if err != nil {
		return nil, err
	}
	if !canViewSubmission(input.Actor, *submission) {
		return nil, submissionNotFound()
	}
	sourceCode, err := u.storage.GetSourceCode(ctx, submission.SourceCodeKey)
	if err != nil {
		return nil, err
	}

	detail := &domain.SubmissionDetail{
		Submission:  *submission,
		SourceCode:  sourceCode,
		CaseResults: []domain.SubmissionCaseResult{},
	}
	// The worker commits results and the terminal status together. If judging
	// finishes after this read, return the observed pending/running state with
	// an empty array; the next poll will fetch the committed results.
	if submission.Status == domain.SubmissionStatusPending || submission.Status == domain.SubmissionStatusRunning {
		return detail, nil
	}
	results, err := u.submissions.ListCaseResults(ctx, submission.ID)
	if err != nil {
		return nil, err
	}
	detail.CaseResults = append(detail.CaseResults, results...)
	return detail, nil
}

type ListSubmissionsInput struct {
	Actor         auth.Actor
	ProblemID     *int64 // nil lists submissions to every problem
	Limit         int
	LimitProvided bool
	Offset        int
}

func (u *SubmissionUsecase) List(ctx context.Context, input ListSubmissionsInput) ([]domain.Submission, error) {
	if err := validateActor(input.Actor); err != nil {
		return nil, err
	}

	limit := input.Limit
	if !input.LimitProvided {
		limit = defaultSubmissionLimit
	}
	if limit <= 0 || limit > maxSubmissionLimit {
		return nil, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if input.Offset < 0 {
		return nil, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}
	if input.ProblemID != nil && *input.ProblemID <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}

	return u.submissions.ListByUserID(ctx, repository.ListSubmissionsParams{
		UserID:    input.Actor.UserID,
		ProblemID: input.ProblemID,
		Limit:     limit,
		Offset:    input.Offset,
	})
}

type ListRecentSubmissionsInput struct {
	Limit         int
	LimitProvided bool
	Offset        int
}

// ListRecentPublic lists recent public submissions. Anyone may call it; the
// result carries no source and no user id.
func (u *SubmissionUsecase) ListRecentPublic(ctx context.Context, input ListRecentSubmissionsInput) ([]domain.PublicSubmission, error) {
	limit := input.Limit
	if !input.LimitProvided {
		limit = defaultSubmissionLimit
	}
	if limit <= 0 || limit > maxSubmissionLimit {
		return nil, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if input.Offset < 0 {
		return nil, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}

	return u.submissions.ListRecentPublic(ctx, repository.ListRecentSubmissionsParams{
		Limit:  limit,
		Offset: input.Offset,
	})
}

func newSourceCodeKey(userID string, problemID int64, language domain.SubmissionLanguage) (string, error) {
	token, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("submissions/%s/problems/%d/%s%s", userID, problemID, token, sourceExtension(language)), nil
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
