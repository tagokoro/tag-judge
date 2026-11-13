package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type SubmissionRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewSubmissionRepository(db *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{db: db, queries: sqlc.New(db)}
}

type submissionOutboxPayload struct {
	SubmissionID  string                    `json:"submissionId"`
	Language      domain.SubmissionLanguage `json:"language"`
	SourceCodeKey string                    `json:"sourceCodeKey"`
	TestCasesKey  string                    `json:"testCasesKey"`
	TimeLimitMS   int                       `json:"timeLimitMs"`
}

// SubmissionOutboxPublisher drains committed messages. It intentionally marks
// an item dispatched only after SendMessage succeeds; a crash after SendMessage
// can duplicate a job, which is safe because the worker result sink is
// idempotent.
type SubmissionOutboxPublisher struct {
	db         *pgxpool.Pool
	dispatcher repository.ExecutionDispatcher
	logger     *slog.Logger
}

func NewSubmissionOutboxPublisher(db *pgxpool.Pool, dispatcher repository.ExecutionDispatcher, logger *slog.Logger) *SubmissionOutboxPublisher {
	return &SubmissionOutboxPublisher{db: db, dispatcher: dispatcher, logger: logger}
}

func (p *SubmissionOutboxPublisher) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	p.publish(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.publish(ctx)
		}
	}
}

func (p *SubmissionOutboxPublisher) publish(ctx context.Context) {
	if _, err := p.PublishPending(ctx, 16); err != nil && p.logger != nil {
		p.logger.Error("publish submission outbox", "error", err)
	}
}

func (p *SubmissionOutboxPublisher) PublishPending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	published := 0
	for range limit {
		tx, err := p.db.Begin(ctx)
		if err != nil {
			return published, fmt.Errorf("begin outbox publish transaction: %w", err)
		}
		var submissionID int64
		var payloadBytes []byte
		var attempts int
		err = tx.QueryRow(ctx, `
SELECT submission_id, payload, attempts
FROM submission_job_outbox
WHERE dispatched_at IS NULL AND available_at <= now()
ORDER BY submission_id
FOR UPDATE SKIP LOCKED
LIMIT 1`).Scan(&submissionID, &payloadBytes, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return published, nil
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return published, fmt.Errorf("claim submission outbox record: %w", err)
		}

		job, jobErr := executionJobFromOutboxPayload(payloadBytes)
		if jobErr == nil {
			jobErr = p.dispatcher.Dispatch(ctx, job)
		}
		if jobErr != nil {
			backoff := retryBackoff(attempts)
			_, updateErr := tx.Exec(ctx, `
UPDATE submission_job_outbox
SET attempts = attempts + 1, last_error = $2, available_at = $3, updated_at = now()
WHERE submission_id = $1`, submissionID, jobErr.Error(), time.Now().Add(backoff))
			if updateErr != nil {
				_ = tx.Rollback(ctx)
				return published, fmt.Errorf("record outbox dispatch failure: %w", updateErr)
			}
			if err := tx.Commit(ctx); err != nil {
				return published, fmt.Errorf("commit outbox dispatch failure: %w", err)
			}
			return published, fmt.Errorf("dispatch submission %d: %w", submissionID, jobErr)
		}
		if _, err := tx.Exec(ctx, `
UPDATE submission_job_outbox
SET dispatched_at = now(), last_error = NULL, updated_at = now()
WHERE submission_id = $1`, submissionID); err != nil {
			_ = tx.Rollback(ctx)
			return published, fmt.Errorf("mark outbox dispatched: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return published, fmt.Errorf("commit outbox dispatch: %w", err)
		}
		published++
	}
	return published, nil
}

func executionJobFromOutboxPayload(raw []byte) (repository.ExecutionJob, error) {
	var payload submissionOutboxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return repository.ExecutionJob{}, fmt.Errorf("decode outbox payload: %w", err)
	}
	reference, err := repository.NewSubmissionExecutionReference(payload.SubmissionID)
	if err != nil {
		return repository.ExecutionJob{}, err
	}
	mode, err := repository.NewProblemExecution(payload.TestCasesKey, payload.TimeLimitMS)
	if err != nil {
		return repository.ExecutionJob{}, err
	}
	return repository.NewExecutionJob(reference, payload.Language, payload.SourceCodeKey, mode)
}

func (r *SubmissionRepository) FindByID(ctx context.Context, id int64) (*domain.Submission, error) {
	row, err := r.queries.GetSubmission(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("SUBMISSION_NOT_FOUND", "submission not found")
	}
	if err != nil {
		return nil, err
	}

	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.TestCasesKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	return &submission, nil
}

func (r *SubmissionRepository) ListCaseResults(ctx context.Context, submissionID int64) ([]domain.SubmissionCaseResult, error) {
	rows, err := r.queries.ListSubmissionCaseResults(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	results := make([]domain.SubmissionCaseResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, domain.SubmissionCaseResult{
			TestCaseID:      row.TestCaseID,
			Status:          domain.SubmissionStatus(row.Status),
			ExecutionTimeMS: optionalInt(row.ExecutionTimeMs),
			MemoryKB:        optionalInt(row.MemoryKb),
		})
	}
	return results, nil
}

func (r *SubmissionRepository) ListByUserID(ctx context.Context, params repository.ListSubmissionsParams) ([]domain.Submission, error) {
	rows, err := r.queries.ListUserSubmissions(ctx, sqlc.ListUserSubmissionsParams{
		UserID:       params.UserID,
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	submissions := make([]domain.Submission, 0, len(rows))
	for _, row := range rows {
		submissions = append(submissions, newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.TestCasesKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt))
	}
	return submissions, nil
}

func (r *SubmissionRepository) ListRecentPublic(ctx context.Context, params repository.ListRecentSubmissionsParams) ([]domain.PublicSubmission, error) {
	rows, err := r.queries.ListRecentPublicSubmissions(ctx, sqlc.ListRecentPublicSubmissionsParams{
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	submissions := make([]domain.PublicSubmission, 0, len(rows))
	for _, row := range rows {
		var judgedAt *time.Time
		if row.JudgedAt.Valid {
			value := row.JudgedAt.Time
			judgedAt = &value
		}
		submissions = append(submissions, domain.PublicSubmission{
			ID:              row.ID,
			ProblemID:       row.ProblemID,
			ProblemTitle:    row.ProblemTitle,
			Language:        domain.SubmissionLanguage(row.Language),
			Status:          domain.SubmissionStatus(row.Status),
			Score:           int(row.Score),
			CreatedAt:       row.CreatedAt.Time,
			JudgedAt:        judgedAt,
			SubmitterHandle: row.SubmitterHandle.String,
		})
	}
	return submissions, nil
}

func newSubmission(
	id int64,
	userID string,
	problemID int64,
	language string,
	sourceCodeKey string,
	testCasesKey string,
	status string,
	score int32,
	createdAt pgtype.Timestamptz,
	judgedAt pgtype.Timestamptz,
) domain.Submission {
	var judgedAtTime *time.Time
	if judgedAt.Valid {
		value := judgedAt.Time
		judgedAtTime = &value
	}

	return domain.Submission{
		ID:            id,
		UserID:        userID,
		ProblemID:     problemID,
		Language:      domain.SubmissionLanguage(language),
		SourceCodeKey: sourceCodeKey,
		TestCasesKey:  testCasesKey,
		Status:        domain.SubmissionStatus(status),
		Score:         int(score),
		CreatedAt:     createdAt.Time,
		JudgedAt:      judgedAtTime,
	}
}
