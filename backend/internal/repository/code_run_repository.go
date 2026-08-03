package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateCodeRunParams struct {
	UserID     string
	Language   domain.ExecutionLanguage
	SourceCode string
	Stdin      string
}

type ListCodeRunsParams struct {
	UserID string
	Limit  int
	Offset int
}

type CodeRunRepository interface {
	Create(ctx context.Context, params CreateCodeRunParams) (*domain.CodeRun, error)
	FindByID(ctx context.Context, id int64) (*domain.CodeRun, error)
	ListByUserID(ctx context.Context, params ListCodeRunsParams) ([]domain.CodeRun, error)
}
