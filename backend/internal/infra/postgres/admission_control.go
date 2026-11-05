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
//
// Throughput note: the shared lock row serializes every admission across all
// API processes, so the system-wide admission rate is bounded by roughly one
// request per lock-hold duration. The lock is only held for the single
// combined count query and the insert statements; never for storage, SQS, or
// other network I/O. If this ceiling is ever hit in production, split the
// per-user checks onto per-user advisory locks and keep only the global
// outstanding check on the shared row.
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
// stored before the atomic record+outbox creation. The sequence is resolved via
// pg_get_serial_sequence so renames or a move to IDENTITY columns do not break
// this call.
func (a *AdmissionControl) ReserveCodeRunID(ctx context.Context) (int64, error) {
	var id int64
	if err := a.db.QueryRow(ctx, "SELECT nextval(pg_get_serial_sequence('code_runs', 'id'))").Scan(&id); err != nil {
		return 0, classifyDBError(err, "reserve code run id")
	}
	return id, nil
}

func (a *AdmissionControl) Validate() error {
	return a.limits.Validate()
}

func (a *AdmissionControl) CreateSubmission(ctx context.Context, params repository.CreateSubmissionWithOutboxParams, idempotency *repository.IdempotencyKeyInfo) (*domain.Submission, error) {
	if params.TimeLimitMS <= 0 {
		return nil, apperror.InvalidInput("INVALID_TIME_LIMIT", "submission time limit must be positive")
	}

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
				return nil, classifyCommitError()
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
		return nil, classifyCommitError()
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
				return nil, classifyCommitError()
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
	row, err := a.queries.WithTx(tx).CreateCodeRunWithID(ctx, sqlc.CreateCodeRunWithIDParams{
		ID:         codeRunID,
		UserID:     userIDUUID,
		Language:   string(params.Language),
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
	})
	if err != nil {
		return nil, classifyDBError(err, "create code run")
	}
	rowID = row.ID
	rowUserID = row.UserID
	rowLanguage = row.Language
	rowStdin = row.Stdin
	rowCreatedAt = row.CreatedAt.Time

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
		return nil, classifyCommitError()
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

// admissionCountsResult holds the three admission checks evaluated in a single
// query so the global row lock is held for one round trip instead of four.
type admissionCountsResult struct {
	rateCount         int64
	rateRetryAfter    *int
	userOutstanding   int64
	globalOutstanding int64
}

func (a *AdmissionControl) checkAdmission(ctx context.Context, tx pgx.Tx, userID string) error {
	counts, err := a.admissionCounts(ctx, tx, userID)
	if err != nil {
		return err
	}

	if counts.rateCount >= int64(a.limits.UserRateLimit) {
		return apperror.RateLimit(
			"USER_RATE_LIMIT_EXCEEDED",
			fmt.Sprintf("user creation rate limit exceeded: %d per %v", a.limits.UserRateLimit, a.limits.UserRateWindow),
			smallestRetryAfter(a.limits.UserRateWindow, counts.rateRetryAfter),
		)
	}

	if counts.userOutstanding >= int64(a.limits.UserOutstandingLimit) {
		return apperror.RateLimit(
			"USER_OUTSTANDING_LIMIT_EXCEEDED",
			fmt.Sprintf("user outstanding execution limit exceeded: %d", a.limits.UserOutstandingLimit),
			1,
		)
	}

	if counts.globalOutstanding >= int64(a.limits.GlobalOutstandingLimit) {
		return apperror.OverCapacity(
			"GLOBAL_OUTSTANDING_LIMIT_EXCEEDED",
			fmt.Sprintf("global outstanding execution limit exceeded: %d", a.limits.GlobalOutstandingLimit),
			5,
		)
	}

	return nil
}

// admissionCounts evaluates the per-user rolling rate, per-user outstanding,
// and global outstanding counts in one query. All three decisions must observe
// the same snapshot while holding the admission lock; consolidating them keeps
// the lock hold time to a single round trip.
func (a *AdmissionControl) admissionCounts(ctx context.Context, tx pgx.Tx, userID string) (admissionCountsResult, error) {
	var counts admissionCountsResult
	err := tx.QueryRow(ctx, `
		SELECT rate.rate_count, rate.retry_after, outstanding.user_outstanding, outstanding.global_outstanding
		FROM (
			SELECT count(*) AS rate_count,
				ceil(extract(epoch FROM ($2::interval - (now() - min(created_at)))))::int AS retry_after
			FROM (
				SELECT created_at FROM submissions
				WHERE user_id = $1::uuid AND created_at >= now() - $2::interval
				UNION ALL
				SELECT created_at FROM code_runs
				WHERE user_id = $1::uuid AND created_at >= now() - $2::interval
			) recent
		) rate
		CROSS JOIN (
			SELECT
				(SELECT count(*) FROM submissions
					WHERE user_id = $1::uuid AND status IN ('pending', 'running'))
					+ (SELECT count(*) FROM code_runs
					WHERE user_id = $1::uuid AND status IN ('pending', 'running')) AS user_outstanding,
				(SELECT count(*) FROM submissions WHERE status IN ('pending', 'running'))
					+ (SELECT count(*) FROM code_runs WHERE status IN ('pending', 'running')) AS global_outstanding
		) outstanding`,
		userID, a.limits.UserRateWindow,
	).Scan(&counts.rateCount, &counts.rateRetryAfter, &counts.userOutstanding, &counts.globalOutstanding)
	if err != nil {
		return admissionCountsResult{}, classifyDBError(err, "count admission limits")
	}
	return counts, nil
}

func smallestRetryAfter(window time.Duration, values ...*int) int {
	smallest := int(window.Seconds())
	found := false
	for _, v := range values {
		if v == nil {
			continue
		}
		found = true
		if *v < smallest {
			smallest = *v
		}
	}
	if !found {
		return int(window.Seconds())
	}
	if smallest < 1 {
		return 1
	}
	return smallest
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

// classifyCommitError maps a failed admission commit to a transient error.
// The transaction runs at READ COMMITTED, so serialization failures do not
// occur; a failed commit means the connection or the statement was lost, and
// the client must retry with the same idempotency key.
func classifyCommitError() error {
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
