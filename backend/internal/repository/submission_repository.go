package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateSubmissionParams struct {
	UserID        string
	ProblemID     int64
	Language      domain.SubmissionLanguage
	SourceCodeKey string
	Status        domain.SubmissionStatus
	Score         int
}

type ListSubmissionsParams struct {
	UserID string
	Limit  int
	Offset int
}

type SubmissionRepository interface {
	Create(ctx context.Context, params CreateSubmissionParams) (*domain.Submission, error)
	FindByID(ctx context.Context, id int64) (*domain.Submission, error)
	ListByUserID(ctx context.Context, params ListSubmissionsParams) ([]domain.Submission, error)
}
