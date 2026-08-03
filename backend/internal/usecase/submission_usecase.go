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
	submissions repository.SubmissionRepository
	problems    repository.PublicProblemReader
	storage     repository.SourceCodeStorage
	dispatcher  repository.ExecutionDispatcher
}

func NewSubmissionUsecase(
	submissions repository.SubmissionRepository,
	problems repository.PublicProblemReader,
	storage repository.SourceCodeStorage,
	dispatcher repository.ExecutionDispatcher,
) *SubmissionUsecase {
	return &SubmissionUsecase{
		submissions: submissions,
		problems:    problems,
		storage:     storage,
		dispatcher:  dispatcher,
	}
}

type CreateSubmissionInput struct {
	Actor      auth.Actor
	ProblemID  int64
	Language   domain.SubmissionLanguage
	SourceCode string
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
	if strings.TrimSpace(input.SourceCode) == "" {
		return nil, apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code must not be empty")
	}

	// Problem access policy belongs here so private/group access can be added
	// without coupling the handler or persistence adapter to authorization.
	problem, err := u.problems.FindPublicByID(ctx, input.ProblemID)
	if err != nil {
		return nil, err
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

	submission, err := u.submissions.Create(ctx, repository.CreateSubmissionParams{
		UserID:        input.Actor.UserID,
		ProblemID:     input.ProblemID,
		Language:      input.Language,
		SourceCodeKey: sourceCodeKey,
		Status:        domain.SubmissionStatusPending,
		Score:         0,
	})
	if err != nil {
		return nil, err
	}

	reference, err := repository.NewSubmissionExecutionReference(fmt.Sprintf("%d", submission.ID))
	if err != nil {
		return nil, err
	}
	mode, err := repository.NewProblemExecution(testCasesKey(problem.ID), problem.TimeLimitMS)
	if err != nil {
		return nil, err
	}
	job, err := repository.NewExecutionJob(reference, input.Language, sourceCodeKey, mode)
	if err != nil {
		return nil, err
	}
	if err := u.dispatcher.Dispatch(ctx, job); err != nil {
		return nil, err
	}

	return submission, nil
}

type GetSubmissionInput struct {
	Actor        auth.Actor
	SubmissionID int64
}

func (u *SubmissionUsecase) Get(ctx context.Context, input GetSubmissionInput) (*domain.Submission, error) {
	if err := validateActor(input.Actor); err != nil {
		return nil, err
	}
	if input.SubmissionID <= 0 {
		return nil, apperror.InvalidInput("INVALID_SUBMISSION_ID", "submission_id must be a positive integer")
	}

	submission, err := u.submissions.FindByID(ctx, input.SubmissionID)
	if err != nil {
		return nil, err
	}
	if submission.UserID != input.Actor.UserID && !input.Actor.Role.CanViewAnySubmission() {
		return nil, apperror.Forbidden("SUBMISSION_FORBIDDEN", "submission is not accessible")
	}

	return submission, nil
}

type ListSubmissionsInput struct {
	Actor         auth.Actor
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

	return u.submissions.ListByUserID(ctx, repository.ListSubmissionsParams{
		UserID: input.Actor.UserID,
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

func testCasesKey(problemID int64) string {
	return fmt.Sprintf("problems/%d/testcases.json", problemID)
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
