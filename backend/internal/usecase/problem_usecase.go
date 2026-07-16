package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
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
