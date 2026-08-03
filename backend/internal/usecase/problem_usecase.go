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

type CreateProblemInput struct {
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
	TestCases     []CreateTestCaseInput
}

type CreateTestCaseInput struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
}

func (u *ProblemUsecase) CreateProblem(ctx context.Context, actor auth.Actor, input CreateProblemInput) (*domain.Problem, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if !actor.Role.CanCreateProblem() {
		return nil, apperror.Forbidden("FORBIDDEN", "author or admin role required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, apperror.InvalidInput("INVALID_TITLE", "title is required")
	}
	if strings.TrimSpace(input.Statement) == "" {
		return nil, apperror.InvalidInput("INVALID_STATEMENT", "statement is required")
	}
	if input.TimeLimitMS <= 0 || input.TimeLimitMS > math.MaxInt32 {
		return nil, apperror.InvalidInput("INVALID_TIME_LIMIT", "time_limit_ms must be between 1 and 2147483647")
	}
	if input.MemoryLimitKB <= 0 || input.MemoryLimitKB > math.MaxInt32 {
		return nil, apperror.InvalidInput("INVALID_MEMORY_LIMIT", "memory_limit_kb must be between 1 and 2147483647")
	}
	if !input.Visibility.Valid() {
		return nil, apperror.InvalidInput("INVALID_VISIBILITY", "visibility must be public, private, or group")
	}
	if len(input.TestCases) == 0 {
		return nil, apperror.InvalidInput("INVALID_TEST_CASES", "at least one test case is required")
	}

	testCases := make([]domain.TestCase, 0, len(input.TestCases))
	for _, testCase := range input.TestCases {
		testCases = append(testCases, domain.TestCase{
			Input:          testCase.Input,
			ExpectedOutput: testCase.ExpectedOutput,
			IsSample:       testCase.IsSample,
			Score:          1,
		})
	}

	return u.problems.Create(ctx, repository.CreateProblemParams{
		Title:         input.Title,
		Statement:     input.Statement,
		TimeLimitMS:   input.TimeLimitMS,
		MemoryLimitKB: input.MemoryLimitKB,
		Visibility:    input.Visibility,
		CreatedBy:     actor.UserID,
		TestCases:     testCases,
	})
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
