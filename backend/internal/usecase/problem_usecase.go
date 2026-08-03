package usecase

import (
	"context"
	"math"
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
	problems  repository.ProblemRepository
	testCases repository.TestCaseCounter
}

func NewProblemUsecase(problems repository.ProblemRepository, testCases repository.TestCaseCounter) *ProblemUsecase {
	return &ProblemUsecase{problems: problems, testCases: testCases}
}

type ListProblemsInput struct {
	Limit         int
	LimitProvided bool
	Offset        int
}

type CreateProblemInput struct {
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
}

type UpdateProblemInput struct {
	Title         *string
	Statement     *string
	TimeLimitMS   *int
	MemoryLimitKB *int
	Visibility    *domain.ProblemVisibility
}

func (u *ProblemUsecase) CreateProblem(ctx context.Context, actor auth.Actor, input CreateProblemInput) (*domain.Problem, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if !actor.Role.CanCreateProblem() {
		return nil, apperror.Forbidden("FORBIDDEN", "author or admin role required")
	}
	if err := validateProblem(input.Title, input.Statement, input.TimeLimitMS, input.MemoryLimitKB, input.Visibility); err != nil {
		return nil, err
	}
	if input.Visibility == domain.ProblemVisibilityPublic {
		return nil, apperror.InvalidInput("PUBLIC_PROBLEM_REQUIRES_TEST_CASE", "create the problem as private or group, add a test case, then make it public")
	}

	return u.problems.Create(ctx, repository.CreateProblemParams{
		Title:         input.Title,
		Statement:     input.Statement,
		TimeLimitMS:   input.TimeLimitMS,
		MemoryLimitKB: input.MemoryLimitKB,
		Visibility:    input.Visibility,
		CreatedBy:     actor.UserID,
	})
}

func (u *ProblemUsecase) UpdateProblem(ctx context.Context, actor auth.Actor, id int64, input UpdateProblemInput) (*domain.Problem, error) {
	if err := validateProblemManager(actor); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	if input.Title == nil && input.Statement == nil && input.TimeLimitMS == nil && input.MemoryLimitKB == nil && input.Visibility == nil {
		return nil, apperror.InvalidInput("EMPTY_UPDATE", "at least one field is required")
	}

	problem, err := u.problems.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := authorizeProblemManager(actor, *problem); err != nil {
		return nil, err
	}

	title := problem.Title
	statement := problem.Statement
	timeLimitMS := problem.TimeLimitMS
	memoryLimitKB := problem.MemoryLimitKB
	visibility := problem.Visibility
	if input.Title != nil {
		title = *input.Title
	}
	if input.Statement != nil {
		statement = *input.Statement
	}
	if input.TimeLimitMS != nil {
		timeLimitMS = *input.TimeLimitMS
	}
	if input.MemoryLimitKB != nil {
		memoryLimitKB = *input.MemoryLimitKB
	}
	if input.Visibility != nil {
		visibility = *input.Visibility
	}
	if err := validateProblem(title, statement, timeLimitMS, memoryLimitKB, visibility); err != nil {
		return nil, err
	}
	if visibility == domain.ProblemVisibilityPublic {
		count, err := u.testCases.CountByProblemID(ctx, id)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, apperror.InvalidInput("PUBLIC_PROBLEM_REQUIRES_TEST_CASE", "public problem requires at least one test case")
		}
	}

	return u.problems.Update(ctx, repository.UpdateProblemParams{
		ID:            id,
		Title:         title,
		Statement:     statement,
		TimeLimitMS:   timeLimitMS,
		MemoryLimitKB: memoryLimitKB,
		Visibility:    visibility,
	})
}

func (u *ProblemUsecase) DeleteProblem(ctx context.Context, actor auth.Actor, id int64) error {
	if err := validateProblemManager(actor); err != nil {
		return err
	}
	if id <= 0 {
		return apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	problem, err := u.problems.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := authorizeProblemManager(actor, *problem); err != nil {
		return err
	}
	return u.problems.Delete(ctx, id)
}

func validateProblem(title, statement string, timeLimitMS, memoryLimitKB int, visibility domain.ProblemVisibility) error {
	if strings.TrimSpace(title) == "" {
		return apperror.InvalidInput("INVALID_TITLE", "title is required")
	}
	if strings.TrimSpace(statement) == "" {
		return apperror.InvalidInput("INVALID_STATEMENT", "statement is required")
	}
	if timeLimitMS <= 0 || timeLimitMS > math.MaxInt32 {
		return apperror.InvalidInput("INVALID_TIME_LIMIT", "time_limit_ms must be between 1 and 2147483647")
	}
	if memoryLimitKB <= 0 || memoryLimitKB > math.MaxInt32 {
		return apperror.InvalidInput("INVALID_MEMORY_LIMIT", "memory_limit_kb must be between 1 and 2147483647")
	}
	if !visibility.Valid() {
		return apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public, private, or group")
	}
	return nil
}

func validateProblemManager(actor auth.Actor) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	if !actor.Role.CanCreateProblem() {
		return apperror.Forbidden("FORBIDDEN", "author or admin role required")
	}
	return nil
}

func authorizeProblemManager(actor auth.Actor, problem domain.Problem) error {
	if actor.Role != domain.UserRoleAdmin && problem.CreatedBy != actor.UserID {
		return apperror.Forbidden("PROBLEM_MANAGE_FORBIDDEN", "problem owner or admin role required")
	}
	return nil
}

func (u *ProblemUsecase) ListPublicProblems(ctx context.Context, input ListProblemsInput) ([]domain.Problem, error) {
	limit := input.Limit
	if !input.LimitProvided {
		limit = defaultProblemLimit
	}
	if limit <= 0 || limit > maxProblemLimit {
		return nil, apperror.InvalidInput("INVALID_LIMIT", "limit must be between 1 and 100")
	}
	if input.Offset < 0 {
		return nil, apperror.InvalidInput("INVALID_OFFSET", "offset must be greater than or equal to 0")
	}

	return u.problems.ListPublic(ctx, repository.ListProblemsParams{
		Limit:  limit,
		Offset: input.Offset,
	})
}

func (u *ProblemUsecase) GetPublicProblem(ctx context.Context, id int64) (*domain.Problem, error) {
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}

	return u.problems.FindPublicByID(ctx, id)
}
