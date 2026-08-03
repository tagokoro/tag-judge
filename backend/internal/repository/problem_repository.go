package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type ListProblemsParams struct {
	Limit  int
	Offset int
}

type ProblemMutationParams struct {
	Title         string
	Statement     string
	TimeLimitMS   int
	MemoryLimitKB int
	Visibility    domain.ProblemVisibility
	CreatedBy     string
}

type TestCaseMutationParams struct {
	ProblemID      int64
	Input          string
	ExpectedOutput string
	IsSample       bool
	Score          int
}

type CreateSubmissionParams struct {
	UserID     string
	ProblemID  int64
	Language   string
	SourceCode string
}

type ProblemRepository interface {
	ListPublic(ctx context.Context, params ListProblemsParams) ([]domain.Problem, error)
	ListVisible(ctx context.Context, params ListProblemsParams, actorUserID string, canSeeAll bool) ([]domain.Problem, error)
	FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error)
	FindVisibleByID(ctx context.Context, id int64, actorUserID string, canSeeAll bool) (*domain.Problem, error)
	Create(ctx context.Context, params ProblemMutationParams) (*domain.Problem, error)
	Update(ctx context.Context, id int64, params ProblemMutationParams) (*domain.Problem, error)
	Delete(ctx context.Context, id int64) error
	ListTestCases(ctx context.Context, problemID int64) ([]domain.TestCase, error)
	CreateTestCase(ctx context.Context, params TestCaseMutationParams) (*domain.TestCase, error)
	UpdateTestCase(ctx context.Context, id int64, params TestCaseMutationParams) (*domain.TestCase, error)
	DeleteTestCase(ctx context.Context, problemID int64, id int64) error
	CreateSubmission(ctx context.Context, params CreateSubmissionParams) (*domain.Submission, error)
	ListSubmissionsByProblem(ctx context.Context, problemID int64, userID string, canSeeAll bool) ([]domain.Submission, error)
	FindSubmissionByID(ctx context.Context, id int64) (*domain.Submission, error)
}
