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
}

type UpdateProblemParams struct {
	ID            int64
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
}

type PublicProblemReader interface {
	FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error)
}

type TestCaseSnapshotProblemRepository interface {
	ProblemRepository
	SetCurrentTestCasesKey(ctx context.Context, problemID int64, key string) error
}

type ProblemRepository interface {
	PublicProblemReader
	ListPublic(ctx context.Context, params ListProblemsParams) ([]domain.Problem, error)
	FindByID(ctx context.Context, id int64) (*domain.Problem, error)
	Create(ctx context.Context, params CreateProblemParams) (*domain.Problem, error)
	Update(ctx context.Context, params UpdateProblemParams) (*domain.Problem, error)
	Delete(ctx context.Context, id int64) error
}
