package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type ListProblemsParams struct {
	Limit  int
	Offset int
}

type ListProblemsByCreatorParams struct {
	CreatedBy string
	Limit     int
	Offset    int
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

// ProblemReader finds a problem whatever its visibility; callers decide who
// may see it.
type ProblemReader interface {
	FindByID(ctx context.Context, id int64) (*domain.Problem, error)
}

type TestCaseSnapshotProblemRepository interface {
	ProblemRepository
	SetCurrentTestCasesKey(ctx context.Context, problemID int64, key string) error
}

type ProblemRepository interface {
	ProblemReader
	ListPublic(ctx context.Context, params ListProblemsParams) ([]domain.Problem, error)
	ListByCreator(ctx context.Context, params ListProblemsByCreatorParams) ([]domain.Problem, error)
	Create(ctx context.Context, params CreateProblemParams) (*domain.Problem, error)
	Update(ctx context.Context, params UpdateProblemParams) (*domain.Problem, error)
	Delete(ctx context.Context, id int64) error
}
