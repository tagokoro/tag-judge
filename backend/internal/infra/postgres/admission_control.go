package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// AdmissionLimits configures the shared admission controller. All values must
// be positive.
type AdmissionLimits struct {
	UserRateLimit          int
	UserRateWindow         time.Duration
	UserOutstandingLimit   int
	GlobalOutstandingLimit int
	LockTimeout            time.Duration
}

func (l AdmissionLimits) Validate() error {
	if l.UserRateLimit <= 0 {
		return fmt.Errorf("admission user rate limit must be positive")
	}
	if l.UserRateWindow <= 0 {
		return fmt.Errorf("admission user rate window must be positive")
	}
	if l.UserOutstandingLimit <= 0 {
		return fmt.Errorf("admission user outstanding limit must be positive")
	}
	if l.GlobalOutstandingLimit <= 0 {
		return fmt.Errorf("admission global outstanding limit must be positive")
	}
	if l.LockTimeout <= 0 {
		return fmt.Errorf("admission lock timeout must be positive")
	}
	return nil
}

// AdmissionControl serializes execution creation through a single DB lock row,
// checks per-user and global limits, persists durable outbox records, and
// records idempotency keys. All creation paths for submissions and code runs
// must go through it.
type AdmissionControl struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
	limits  AdmissionLimits
}

func NewAdmissionControl(db *pgxpool.Pool, limits AdmissionLimits) *AdmissionControl {
	return &AdmissionControl{db: db, queries: sqlc.New(db), limits: limits}
}

// ReserveCodeRunID allocates the next code_runs id outside of any admission
// transaction so the source-code storage key can be computed and the artifact
// stored before the atomic record+outbox creation.
func (a *AdmissionControl) ReserveCodeRunID(ctx context.Context) (int64, error) {
	var id int64
	if err := a.db.QueryRow(ctx, "SELECT nextval('code_runs_id_seq')").Scan(&id); err != nil {
		return 0, classifyDBError(err, "reserve code run id")
	}
	return id, nil
}

func (a *AdmissionControl) Validate() error {
	return a.limits.Validate()
}

func (a *AdmissionControl) CreateSubmission(ctx context.Context, params repository.CreateSubmissionWithOutboxParams, idempotency *repository.IdempotencyKeyInfo) (*domain.Submission, error) {
	tx, err := a.beginAdmissionTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if idempotency != nil {
		existing, err := a.findIdempotentSubmission(ctx, tx, params.UserID, idempotency.Namespace, idempotency.Key, idempotency.Fingerprint)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if err := tx.Commit(ctx); err != nil {
				return nil, classifyCommitError(err)
			}
			return existing, nil
		}
	}

	if err := a.checkAdmission(ctx, tx, params.UserID); err != nil {
		return nil, err
	}

	row, err := a.queries.WithTx(tx).CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      string(params.Language),
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        string(params.Status),
		Score:         int32(params.Score),
	})
	if err != nil {
		return nil, classifyDBError(err, "create submission")
	}

	payload, err := json.Marshal(submissionOutboxPayload{
		SubmissionID:  fmt.Sprintf("%d", row.ID),
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		TimeLimitMS:   params.TimeLimitMS,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal submission outbox payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO submission_job_outbox (submission_id, payload) VALUES ($1, $2::jsonb)`, row.ID, payload); err != nil {
		return nil, classifyDBError(err, "create submission outbox record")
	}

	if idempotency != nil {
		if err := a.insertIdempotencySubmission(ctx, tx, params.UserID, idempotency, row.ID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, classifyCommitError(err)
	}

	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.TestCasesKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	return &submission, nil
}

func (a *AdmissionControl) CreateCodeRun(ctx context.Context, params repository.CreateCodeRunParams, idempotency *repository.IdempotencyKeyInfo, timeLimitMS int, codeRunID int64) (*domain.CodeRun, error) {
	if timeLimitMS <= 0 {
		return nil, apperror.InvalidInput("INVALID_TIME_LIMIT", "code run time limit must be positive")
	}
	if codeRunID <= 0 {
		return nil, apperror.Internal("reserved code run id must be positive")
	}
	if strings.TrimSpace(params.SourceCodeKey) == "" {
		return nil, apperror.Internal("source code key must be set before admission")
	}

	userIDUUID, err := parseUUID(params.UserID)
	if err != nil {
		return nil, err
	}

	tx, err := a.beginAdmissionTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if idempotency != nil {
		existing, err := a.findIdempotentCodeRun(ctx, tx, params.UserID, idempotency.Namespace, idempotency.Key, idempotency.Fingerprint)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if err := tx.Commit(ctx); err != nil {
				return nil, classifyCommitError(err)
			}
			return existing, nil
		}
	}

	if err := a.checkAdmission(ctx, tx, params.UserID); err != nil {
		return nil, err
	}

	var rowID int64
	var rowUserID string
	var rowLanguage, rowStdin string
	var rowCreatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO code_runs (id, user_id, language, source_code, stdin, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING id, user_id::text AS user_id, language, stdin, created_at`,
		codeRunID, userIDUUID, string(params.Language), params.SourceCode, params.Stdin,
	).Scan(&rowID, &rowUserID, &rowLanguage, &rowStdin, &rowCreatedAt)
	if err != nil {
		return nil, classifyDBError(err, "create code run")
	}

	payload, err := json.Marshal(codeRunOutboxPayload{
		CodeRunID:     fmt.Sprintf("%d", rowID),
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		Stdin:         rowStdin,
		TimeLimitMS:   timeLimitMS,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal code run outbox payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO code_run_job_outbox (code_run_id, payload) VALUES ($1, $2::jsonb)`, rowID, payload); err != nil {
		return nil, classifyDBError(err, "create code run outbox record")
	}

	if idempotency != nil {
		if err := a.insertIdempotencyCodeRun(ctx, tx, params.UserID, idempotency, rowID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, classifyCommitError(err)
	}

	codeRun := domain.CodeRun{
		ID:         rowID,
		UserID:     rowUserID,
		Language:   domain.ExecutionLanguage(rowLanguage),
		SourceCode: params.SourceCode,
		Stdin:      rowStdin,
		Status:     domain.CodeRunStatusPending,
		CreatedAt:  rowCreatedAt,
	}
	return &codeRun, nil
}

func (a *AdmissionControl) beginAdmissionTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := a.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, classifyDBError(err, "begin admission transaction")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", a.limits.LockTimeout.Milliseconds())); err != nil {
		_ = tx.Rollback(ctx)
		return nil, classifyDBError(err, "set admission lock timeout")
	}
	var lockID int
	err = tx.QueryRow(ctx, "SELECT id FROM admission_control_lock WHERE id = 1 FOR UPDATE").Scan(&lockID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return nil, apperror.Transient("TRANSIENT_DATABASE_ERROR", "admission lock row is missing", 1)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, classifyDBError(err, "acquire admission lock")
	}
	return tx, nil
}

func (a *AdmissionControl) checkAdmission(ctx context.Context, tx pgx.Tx, userID string) error {
	userRateCount, retryAfterSeconds, err := a.userAcceptedCount(ctx, tx, userID)
	if err != nil {
		return err
	}
	if userRateCount >= a.limits.UserRateLimit {
		return apperror.RateLimit(
			"USER_RATE_LIMIT_EXCEEDED",
			fmt.Sprintf("user creation rate limit exceeded: %d per %v", a.limits.UserRateLimit, a.limits.UserRateWindow),
			retryAfterSeconds,
		)
	}

	userOutstanding, err := a.userOutstandingCount(ctx, tx, userID)
	if err != nil {
		return err
	}
	if userOutstanding >= a.limits.UserOutstandingLimit {
		return apperror.RateLimit(
			"USER_OUTSTANDING_LIMIT_EXCEEDED",
			fmt.Sprintf("user outstanding execution limit exceeded: %d", a.limits.UserOutstandingLimit),
			1,
		)
	}

	globalOutstanding, err := a.globalOutstandingCount(ctx, tx)
	if err != nil {
		return err
	}
	if globalOutstanding >= a.limits.GlobalOutstandingLimit {
		return apperror.OverCapacity(
			"GLOBAL_OUTSTANDING_LIMIT_EXCEEDED",
			fmt.Sprintf("global outstanding execution limit exceeded: %d", a.limits.GlobalOutstandingLimit),
			5,
		)
	}

	return nil
}

func (a *AdmissionControl) userAcceptedCount(ctx context.Context, tx pgx.Tx, userID string) (int, int, error) {
	var subCount, runCount int64
	var subRetryAfter, runRetryAfter *int
	err := tx.QueryRow(ctx, `
		SELECT count(*),
			ceil(extract(epoch from ($2::interval - (now() - min(created_at)))))::int
		FROM submissions
		WHERE user_id = $1::uuid AND created_at >= now() - $2::interval`,
		userID, a.limits.UserRateWindow,
	).Scan(&subCount, &subRetryAfter)
	if err != nil {
		return 0, 0, classifyDBError(err, "count user submission rate")
	}
	err = tx.QueryRow(ctx, `
		SELECT count(*),
			ceil(extract(epoch from ($2::interval - (now() - min(created_at)))))::int
		FROM code_runs
		WHERE user_id = $1::uuid AND created_at >= now() - $2::interval`,
		userID, a.limits.UserRateWindow,
	).Scan(&runCount, &runRetryAfter)
	if err != nil {
		return 0, 0, classifyDBError(err, "count user code run rate")
	}
	total := int(subCount + runCount)
	retryAfter := smallestRetryAfter(a.limits.UserRateWindow, subRetryAfter, runRetryAfter)
	return total, retryAfter, nil
}

func smallestRetryAfter(window time.Duration, values ...*int) int {
	min := int(window.Seconds())
	found := false
	for _, v := range values {
		if v == nil {
			continue
		}
		found = true
		if *v < min {
			min = *v
		}
	}
	if !found {
		return int(window.Seconds())
	}
	if min < 1 {
		return 1
	}
	return min
}

func (a *AdmissionControl) userOutstandingCount(ctx context.Context, tx pgx.Tx, userID string) (int, error) {
	var subCount, runCount int64
	err := tx.QueryRow(ctx, `
		SELECT count(*) FROM submissions
		WHERE user_id = $1::uuid AND status IN ('pending', 'running')`,
		userID,
	).Scan(&subCount)
	if err != nil {
		return 0, classifyDBError(err, "count user outstanding submissions")
	}
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM code_runs
		WHERE user_id = $1::uuid AND status IN ('pending', 'running')`,
		userID,
	).Scan(&runCount)
	if err != nil {
		return 0, classifyDBError(err, "count user outstanding code runs")
	}
	return int(subCount + runCount), nil
}

func (a *AdmissionControl) globalOutstandingCount(ctx context.Context, tx pgx.Tx) (int, error) {
	var subCount, runCount int64
	err := tx.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE status IN ('pending', 'running')`).Scan(&subCount)
	if err != nil {
		return 0, classifyDBError(err, "count global outstanding submissions")
	}
	err = tx.QueryRow(ctx, `SELECT count(*) FROM code_runs WHERE status IN ('pending', 'running')`).Scan(&runCount)
	if err != nil {
		return 0, classifyDBError(err, "count global outstanding code runs")
	}
	return int(subCount + runCount), nil
}

func (a *AdmissionControl) findIdempotentSubmission(ctx context.Context, tx pgx.Tx, userID, namespace, key, fingerprint string) (*domain.Submission, error) {
	submissionID, _, storedFingerprint, err := a.findIdempotency(ctx, tx, userID, namespace, key)
	if err != nil || submissionID == 0 {
		return nil, err
	}
	if storedFingerprint != fingerprint {
		return nil, apperror.Conflict("IDEMPOTENCY_KEY_CONFLICT", "idempotency key was used with a different request body")
	}
	row, err := a.queries.WithTx(tx).GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, classifyDBError(err, "fetch idempotent submission")
	}
	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.TestCasesKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	return &submission, nil
}

func (a *AdmissionControl) findIdempotentCodeRun(ctx context.Context, tx pgx.Tx, userID, namespace, key, fingerprint string) (*domain.CodeRun, error) {
	_, codeRunID, storedFingerprint, err := a.findIdempotency(ctx, tx, userID, namespace, key)
	if err != nil || codeRunID == 0 {
		return nil, err
	}
	if storedFingerprint != fingerprint {
		return nil, apperror.Conflict("IDEMPOTENCY_KEY_CONFLICT", "idempotency key was used with a different request body")
	}
	row, err := a.queries.WithTx(tx).GetCodeRun(ctx, codeRunID)
	if err != nil {
		return nil, classifyDBError(err, "fetch idempotent code run")
	}
	codeRun := newCodeRun(row.ID, row.UserID, row.Language, row.SourceCode, row.Stdin, row.Stdout, row.Stderr, row.Status, row.ExecutionTimeMs, row.MemoryKb, row.CreatedAt)
	return &codeRun, nil
}

func (a *AdmissionControl) findIdempotency(ctx context.Context, tx pgx.Tx, userID, namespace, key string) (int64, int64, string, error) {
	var submissionID, codeRunID pgtype.Int8
	var fingerprint string
	err := tx.QueryRow(ctx, `
		SELECT submission_id, code_run_id, fingerprint
		FROM execution_idempotency_keys
		WHERE user_id = $1::uuid AND namespace = $2 AND key = $3`,
		userID, namespace, key,
	).Scan(&submissionID, &codeRunID, &fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, "", nil
	}
	if err != nil {
		return 0, 0, "", classifyDBError(err, "lookup idempotency key")
	}
	return submissionID.Int64, codeRunID.Int64, fingerprint, nil
}

func (a *AdmissionControl) insertIdempotencySubmission(ctx context.Context, tx pgx.Tx, userID string, info *repository.IdempotencyKeyInfo, submissionID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO execution_idempotency_keys (user_id, namespace, key, fingerprint, submission_id)
		VALUES ($1::uuid, $2, $3, $4, $5)`,
		userID, info.Namespace, info.Key, info.Fingerprint, submissionID,
	)
	if err != nil {
		return classifyIdempotencyInsertError(err)
	}
	return nil
}

func (a *AdmissionControl) insertIdempotencyCodeRun(ctx context.Context, tx pgx.Tx, userID string, info *repository.IdempotencyKeyInfo, codeRunID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO execution_idempotency_keys (user_id, namespace, key, fingerprint, code_run_id)
		VALUES ($1::uuid, $2, $3, $4, $5)`,
		userID, info.Namespace, info.Key, info.Fingerprint, codeRunID,
	)
	if err != nil {
		return classifyIdempotencyInsertError(err)
	}
	return nil
}

func classifyIdempotencyInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperror.Conflict("IDEMPOTENCY_KEY_CONFLICT", "idempotency key was used with a different request body")
	}
	return classifyDBError(err, "record idempotency key")
}

func classifyCommitError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "40001" {
		return apperror.Transient("TRANSIENT_COMMIT_FAILURE", "the request could not be confirmed; retry with the same idempotency key", 1)
	}
	return apperror.Transient("TRANSIENT_COMMIT_FAILURE", "the request could not be confirmed; retry with the same idempotency key", 1)
}

func classifyDBError(err error, operation string) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return apperror.Transient("TRANSIENT_DATABASE_ERROR", "database request timed out", 1)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "55P03", "57014":
			return apperror.Transient("TRANSIENT_DATABASE_ERROR", "database is temporarily unavailable; retry later", 1)
		case "08000", "08003", "08006":
			return apperror.Transient("TRANSIENT_DATABASE_ERROR", "database connection failed", 1)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}
