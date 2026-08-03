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
	defaultProblemLimit = 20
	maxProblemLimit     = 100
)

type ProblemUsecase struct {
	problems repository.ProblemRepository
}

func NewProblemUsecase(problems repository.ProblemRepository) *ProblemUsecase {
	return &ProblemUsecase{problems: problems}
}

type ListProblemsInput struct {
	Limit         int
	LimitProvided bool
	Offset        int
}

func (u *ProblemUsecase) ListPublicProblems(ctx context.Context, input ListProblemsInput) ([]domain.Problem, error) {
	params, err := validateListProblemsInput(input)
	if err != nil {
		return nil, err
	}

	return u.problems.ListPublic(ctx, params)
}

func (u *ProblemUsecase) ListVisibleProblems(ctx context.Context, input ListProblemsInput, actor *auth.Actor) ([]domain.Problem, error) {
	params, err := validateListProblemsInput(input)
	if err != nil {
		return nil, err
	}
	userID, canSeeAll := actorScope(actor)
	return u.problems.ListVisible(ctx, params, userID, canSeeAll)
}

func validateListProblemsInput(input ListProblemsInput) (repository.ListProblemsParams, error) {
	limit := input.Limit
	if !input.LimitProvided {
		limit = defaultProblemLimit
	}
	if limit <= 0 || limit > maxProblemLimit {
		return repository.ListProblemsParams{}, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if input.Offset < 0 {
		return repository.ListProblemsParams{}, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}

	return repository.ListProblemsParams{
		Limit:  limit,
		Offset: input.Offset,
	}, nil
}

func (u *ProblemUsecase) GetPublicProblem(ctx context.Context, id int64) (*domain.Problem, error) {
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}

	return u.problems.FindPublicByID(ctx, id)
}

type ProblemInput struct {
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
}

type TestCaseInput struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
	Score          int
}

type SubmissionInput struct {
	Language   string
	SourceCode string
}

func (u *ProblemUsecase) GetVisibleProblem(ctx context.Context, id int64, actor *auth.Actor) (*domain.Problem, error) {
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	userID, canSeeAll := actorScope(actor)
	return u.problems.FindVisibleByID(ctx, id, userID, canSeeAll)
}

func (u *ProblemUsecase) CreateProblem(ctx context.Context, actor auth.Actor, input ProblemInput) (*domain.Problem, error) {
	if !actor.Role.CanCreateProblem() {
		return nil, apperror.Forbidden("FORBIDDEN", "author or admin role is required")
	}
	params, err := validateProblemInput(input, actor.UserID)
	if err != nil {
		return nil, err
	}
	return u.problems.Create(ctx, params)
}

func (u *ProblemUsecase) UpdateProblem(ctx context.Context, actor auth.Actor, id int64, input ProblemInput) (*domain.Problem, error) {
	problem, err := u.requireProblemManager(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	params, err := validateProblemInput(input, problem.CreatedBy)
	if err != nil {
		return nil, err
	}
	return u.problems.Update(ctx, id, params)
}

func (u *ProblemUsecase) DeleteProblem(ctx context.Context, actor auth.Actor, id int64) error {
	if _, err := u.requireProblemManager(ctx, actor, id); err != nil {
		return err
	}
	return u.problems.Delete(ctx, id)
}

func (u *ProblemUsecase) ListTestCases(ctx context.Context, actor auth.Actor, problemID int64) ([]domain.TestCase, error) {
	if _, err := u.requireProblemManager(ctx, actor, problemID); err != nil {
		return nil, err
	}
	return u.problems.ListTestCases(ctx, problemID)
}

func (u *ProblemUsecase) CreateTestCase(ctx context.Context, actor auth.Actor, problemID int64, input TestCaseInput) (*domain.TestCase, error) {
	if _, err := u.requireProblemManager(ctx, actor, problemID); err != nil {
		return nil, err
	}
	params, err := validateTestCaseInput(problemID, input)
	if err != nil {
		return nil, err
	}
	return u.problems.CreateTestCase(ctx, params)
}

func (u *ProblemUsecase) UpdateTestCase(ctx context.Context, actor auth.Actor, problemID int64, testCaseID int64, input TestCaseInput) (*domain.TestCase, error) {
	if _, err := u.requireProblemManager(ctx, actor, problemID); err != nil {
		return nil, err
	}
	if testCaseID <= 0 {
		return nil, apperror.InvalidInput("INVALID_TEST_CASE_ID", "test_case_id must be a positive integer")
	}
	params, err := validateTestCaseInput(problemID, input)
	if err != nil {
		return nil, err
	}
	return u.problems.UpdateTestCase(ctx, testCaseID, params)
}

func (u *ProblemUsecase) DeleteTestCase(ctx context.Context, actor auth.Actor, problemID int64, testCaseID int64) error {
	if _, err := u.requireProblemManager(ctx, actor, problemID); err != nil {
		return err
	}
	if testCaseID <= 0 {
		return apperror.InvalidInput("INVALID_TEST_CASE_ID", "test_case_id must be a positive integer")
	}
	return u.problems.DeleteTestCase(ctx, problemID, testCaseID)
}

func (u *ProblemUsecase) CreateSubmission(ctx context.Context, actor auth.Actor, problemID int64, input SubmissionInput) (*domain.Submission, error) {
	if _, err := u.GetVisibleProblem(ctx, problemID, &actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Language) != "python" {
		return nil, apperror.InvalidInput("INVALID_LANGUAGE", "language must be python")
	}
	if strings.TrimSpace(input.SourceCode) == "" {
		return nil, apperror.InvalidInput("INVALID_SOURCE_CODE", "source_code is required")
	}
	return u.problems.CreateSubmission(ctx, repository.CreateSubmissionParams{
		UserID:     actor.UserID,
		ProblemID:  problemID,
		Language:   "python",
		SourceCode: input.SourceCode,
	})
}

func (u *ProblemUsecase) ListSubmissionsByProblem(ctx context.Context, actor auth.Actor, problemID int64) ([]domain.Submission, error) {
	problem, err := u.GetVisibleProblem(ctx, problemID, &actor)
	if err != nil {
		return nil, err
	}
	canSeeAll := actor.Role == domain.UserRoleAdmin || (actor.Role.CanCreateProblem() && problem.CreatedBy == actor.UserID)
	return u.problems.ListSubmissionsByProblem(ctx, problemID, actor.UserID, canSeeAll)
}

func (u *ProblemUsecase) GetSubmission(ctx context.Context, actor auth.Actor, id int64) (*domain.Submission, error) {
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_SUBMISSION_ID", "submission_id must be a positive integer")
	}
	submission, err := u.problems.FindSubmissionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	problem, err := u.problems.FindVisibleByID(ctx, submission.ProblemID, actor.UserID, actor.Role == domain.UserRoleAdmin)
	if err != nil {
		return nil, err
	}
	canView := submission.UserID == actor.UserID || actor.Role == domain.UserRoleAdmin || (actor.Role.CanCreateProblem() && problem.CreatedBy == actor.UserID)
	if !canView {
		return nil, apperror.Forbidden("FORBIDDEN", "submission is not accessible")
	}
	return submission, nil
}

func (u *ProblemUsecase) requireProblemManager(ctx context.Context, actor auth.Actor, problemID int64) (*domain.Problem, error) {
	if problemID <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	if !actor.Role.CanCreateProblem() {
		return nil, apperror.Forbidden("FORBIDDEN", "author or admin role is required")
	}
	problem, err := u.problems.FindVisibleByID(ctx, problemID, actor.UserID, actor.Role == domain.UserRoleAdmin)
	if err != nil {
		return nil, err
	}
	if actor.Role != domain.UserRoleAdmin && problem.CreatedBy != actor.UserID {
		return nil, apperror.Forbidden("FORBIDDEN", "problem is not manageable")
	}
	return problem, nil
}

func validateProblemInput(input ProblemInput, createdBy string) (repository.ProblemMutationParams, error) {
	title := strings.TrimSpace(input.Title)
	statement := strings.TrimSpace(input.Statement)
	visibility := input.Visibility
	if visibility == "" {
		visibility = domain.ProblemVisibilityPublic
	}
	if title == "" {
		return repository.ProblemMutationParams{}, apperror.InvalidInput("INVALID_TITLE", "title is required")
	}
	if statement == "" {
		return repository.ProblemMutationParams{}, apperror.InvalidInput("INVALID_STATEMENT", "statement is required")
	}
	if input.TimeLimitMS <= 0 {
		return repository.ProblemMutationParams{}, apperror.InvalidInput("INVALID_TIME_LIMIT", "time_limit_ms must be greater than 0")
	}
	if input.MemoryLimitKB <= 0 {
		return repository.ProblemMutationParams{}, apperror.InvalidInput("INVALID_MEMORY_LIMIT", "memory_limit_kb must be greater than 0")
	}
	if !visibility.Valid() {
		return repository.ProblemMutationParams{}, apperror.InvalidInput("INVALID_VISIBILITY", "visibility is invalid")
	}
	return repository.ProblemMutationParams{
		Title:         title,
		Statement:     input.Statement,
		TimeLimitMS:   input.TimeLimitMS,
		MemoryLimitKB: input.MemoryLimitKB,
		Visibility:    visibility,
		CreatedBy:     createdBy,
	}, nil
}

func validateTestCaseInput(problemID int64, input TestCaseInput) (repository.TestCaseMutationParams, error) {
	if problemID <= 0 {
		return repository.TestCaseMutationParams{}, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	if input.Score <= 0 {
		return repository.TestCaseMutationParams{}, apperror.InvalidInput("INVALID_SCORE", "score must be greater than 0")
	}
	return repository.TestCaseMutationParams{
		ProblemID:      problemID,
		Input:          input.Input,
		ExpectedOutput: input.ExpectedOutput,
		IsSample:       input.IsSample,
		Score:          input.Score,
	}, nil
}

func actorScope(actor *auth.Actor) (string, bool) {
	if actor == nil {
		return "", false
	}
	return actor.UserID, actor.Role == domain.UserRoleAdmin
}
