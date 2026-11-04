package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

func retryBackoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 6 {
		attempts = 6
	}
	return time.Second * time.Duration(1<<attempts)
}

// CodeRunOutboxPublisher drains committed code run execution jobs. It mirrors
// the SubmissionOutboxPublisher behavior so accepted code runs are never lost
// because of a temporary queue failure.
type CodeRunOutboxPublisher struct {
	db         *pgxpool.Pool
	dispatcher repository.ExecutionDispatcher
	logger     *slog.Logger
}

func NewCodeRunOutboxPublisher(db *pgxpool.Pool, dispatcher repository.ExecutionDispatcher, logger *slog.Logger) *CodeRunOutboxPublisher {
	return &CodeRunOutboxPublisher{db: db, dispatcher: dispatcher, logger: logger}
}

func (p *CodeRunOutboxPublisher) Run(ctx context.Context, interval time.Duration) {
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

func (p *CodeRunOutboxPublisher) publish(ctx context.Context) {
	if _, err := p.PublishPending(ctx, 16); err != nil && p.logger != nil {
		p.logger.Error("publish code run outbox", "error", err)
	}
}

func (p *CodeRunOutboxPublisher) PublishPending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	published := 0
	for range limit {
		tx, err := p.db.Begin(ctx)
		if err != nil {
			return published, fmt.Errorf("begin code run outbox publish transaction: %w", err)
		}
		var codeRunID int64
		var payloadBytes []byte
		var attempts int
		err = tx.QueryRow(ctx, `
SELECT code_run_id, payload, attempts
FROM code_run_job_outbox
WHERE dispatched_at IS NULL AND available_at <= now()
ORDER BY code_run_id
FOR UPDATE SKIP LOCKED
LIMIT 1`).Scan(&codeRunID, &payloadBytes, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return published, nil
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return published, fmt.Errorf("claim code run outbox record: %w", err)
		}

		job, jobErr := executionJobFromCodeRunOutboxPayload(payloadBytes)
		if jobErr == nil {
			jobErr = p.dispatcher.Dispatch(ctx, job)
		}
		if jobErr != nil {
			backoff := retryBackoff(attempts)
			_, updateErr := tx.Exec(ctx, `
UPDATE code_run_job_outbox
SET attempts = attempts + 1, last_error = $2, available_at = $3, updated_at = now()
WHERE code_run_id = $1`, codeRunID, jobErr.Error(), time.Now().Add(backoff))
			if updateErr != nil {
				_ = tx.Rollback(ctx)
				return published, fmt.Errorf("record code run outbox dispatch failure: %w", updateErr)
			}
			if err := tx.Commit(ctx); err != nil {
				return published, fmt.Errorf("commit code run outbox dispatch failure: %w", err)
			}
			return published, fmt.Errorf("dispatch code run %d: %w", codeRunID, jobErr)
		}
		if _, err := tx.Exec(ctx, `
UPDATE code_run_job_outbox
SET dispatched_at = now(), last_error = NULL, updated_at = now()
WHERE code_run_id = $1`, codeRunID); err != nil {
			_ = tx.Rollback(ctx)
			return published, fmt.Errorf("mark code run outbox dispatched: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return published, fmt.Errorf("commit code run outbox dispatch: %w", err)
		}
		published++
	}
	return published, nil
}

type codeRunOutboxPayload struct {
	CodeRunID     string                   `json:"codeRunId"`
	Language      domain.ExecutionLanguage `json:"language"`
	SourceCodeKey string                   `json:"sourceCodeKey"`
	Stdin         string                   `json:"stdin"`
	TimeLimitMS   int                      `json:"timeLimitMs"`
}

func executionJobFromCodeRunOutboxPayload(raw []byte) (repository.ExecutionJob, error) {
	var payload codeRunOutboxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return repository.ExecutionJob{}, fmt.Errorf("decode code run outbox payload: %w", err)
	}
	reference, err := repository.NewCodeRunExecutionReference(payload.CodeRunID)
	if err != nil {
		return repository.ExecutionJob{}, err
	}
	mode, err := repository.NewStdinExecution(payload.Stdin, payload.TimeLimitMS)
	if err != nil {
		return repository.ExecutionJob{}, err
	}
	return repository.NewExecutionJob(reference, payload.Language, payload.SourceCodeKey, mode)
}
