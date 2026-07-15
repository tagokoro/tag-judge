package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/internal/domain"
)

type ListProblemsParams struct {
	Limit  int
	Offset int
}

type ProblemRepository interface {
	ListPublic(ctx context.Context, params ListProblemsParams) ([]domain.Problem, error)
	FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error)
}
