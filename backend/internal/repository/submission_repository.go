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
	FindByID(ctx context.Context, id int64) (*domain.Submission, error)
	ListCaseResults(ctx context.Context, submissionID int64) ([]domain.SubmissionCaseResult, error)
	ListByUserID(ctx context.Context, params ListSubmissionsParams) ([]domain.Submission, error)
}

// AdmissionControl creates execution records under a shared admission-control
// transaction. It is the only path that should insert new submissions or code
// runs, because admission must be serialized across API processes.
type AdmissionControl interface {
	CreateSubmission(ctx context.Context, params CreateSubmissionWithOutboxParams, idempotency *IdempotencyKeyInfo) (*domain.Submission, error)
	CreateCodeRun(ctx context.Context, params CreateCodeRunParams, idempotency *IdempotencyKeyInfo, timeLimitMS int, codeRunID int64) (*domain.CodeRun, error)
	ReserveCodeRunID(ctx context.Context) (int64, error)
}
