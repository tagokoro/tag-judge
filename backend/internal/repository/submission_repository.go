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
	TestCasesKey  string
	Status        domain.SubmissionStatus
	Score         int
}

// CreateSubmissionWithOutboxParams describes the immutable job to be sent once
// the submission is committed. The repository persists both records atomically.
type CreateSubmissionWithOutboxParams struct {
	CreateSubmissionParams
	TimeLimitMS int
}

type ListSubmissionsParams struct {
	UserID string
	Limit  int
	Offset int
}

type SubmissionRepository interface {
	Create(ctx context.Context, params CreateSubmissionParams) (*domain.Submission, error)
	CreateWithOutbox(ctx context.Context, params CreateSubmissionWithOutboxParams) (*domain.Submission, error)
	FindByID(ctx context.Context, id int64) (*domain.Submission, error)
	ListByUserID(ctx context.Context, params ListSubmissionsParams) ([]domain.Submission, error)
}
