package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type ListProblemsParams struct {
	Limit  int
	Offset int
}

type CreateProblemParams struct {
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
	CreatedBy     string
	TestCases     []domain.TestCase
}

type ProblemRepository interface {
	ListPublic(ctx context.Context, params ListProblemsParams) ([]domain.Problem, error)
	FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error)
	Create(ctx context.Context, params CreateProblemParams) (*domain.Problem, error)
}
