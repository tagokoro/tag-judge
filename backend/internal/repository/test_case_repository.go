package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateTestCaseParams struct {
	ProblemID      int64
	Input          string
	ExpectedOutput string
	IsSample       bool
	Score          int
}

type UpdateTestCaseParams struct {
	ID             int64
	ProblemID      int64
	Input          string
	ExpectedOutput string
	IsSample       bool
}

type TestCaseCounter interface {
	CountByProblemID(ctx context.Context, problemID int64) (int, error)
}

type TestCaseRepository interface {
	TestCaseCounter
	ListByProblemID(ctx context.Context, problemID int64) ([]domain.TestCase, error)
	FindByID(ctx context.Context, problemID, id int64) (*domain.TestCase, error)
	Create(ctx context.Context, params CreateTestCaseParams) (*domain.TestCase, error)
	Update(ctx context.Context, params UpdateTestCaseParams) (*domain.TestCase, error)
	Delete(ctx context.Context, problemID, id int64) error
}
