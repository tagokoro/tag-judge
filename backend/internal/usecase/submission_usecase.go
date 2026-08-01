package usecase

import (
	"context"
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
	problems    repository.ProblemRepository
}

func NewSubmissionUsecase(submissions repository.SubmissionRepository, problems repository.ProblemRepository) *SubmissionUsecase {
	return &SubmissionUsecase{submissions: submissions, problems: problems}
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
	if _, err := u.problems.FindPublicByID(ctx, input.ProblemID); err != nil {
		return nil, err
	}

	return u.submissions.Create(ctx, repository.CreateSubmissionParams{
		UserID:     input.Actor.UserID,
		ProblemID:  input.ProblemID,
		Language:   input.Language,
		SourceCode: input.SourceCode,
		Status:     domain.SubmissionStatusPending,
		Score:      0,
	})
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

func validateActor(actor auth.Actor) error {
	if actor.UserID == "" || !actor.Role.Valid() {
		return apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	return nil
}
