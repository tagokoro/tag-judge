package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

// Actual PostgreSQL integration tests for admission control, durable CodeRun
// outbox, and idempotency. Set ADMISSION_TEST_DATABASE_URL to a database where
// the test user can create schemas and extensions. The tests never truncate
// shared data; each test uses its own isolated schema.
func TestAdmissionControlPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          3,
		UserRateWindow:         1 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	t.Run("per-user rate limit exact boundary and rolling window", func(t *testing.T) {
		userID := admissionTestUserA
		keyBase := "rate-" + t.Name()
		for i := 0; i < limits.UserRateLimit; i++ {
			_, err := admission.CreateSubmission(ctx, submissionParams(userID, "rate"), idempotency(keyBase, i, submissionParams(userID, "rate")))
			if err != nil {
				t.Fatalf("accepted create %d failed: %v", i, err)
			}
		}
		_, err := admission.CreateSubmission(ctx, submissionParams(userID, "rate-excess"), idempotency(keyBase+"-excess", 0, submissionParams(userID, "rate-excess")))
		if !errors.Is(err, apperror.ErrRateLimit) {
			t.Fatalf("expected rate limit error, got %v", err)
		}
		appErr := err.(*apperror.AppError)
		if appErr.Code != "USER_RATE_LIMIT_EXCEEDED" {
			t.Fatalf("error code = %q, want USER_RATE_LIMIT_EXCEEDED", appErr.Code)
		}
		if appErr.RetryAfter <= 0 || appErr.RetryAfter > int(limits.UserRateWindow.Seconds())+1 {
			t.Fatalf("Retry-After = %d", appErr.RetryAfter)
		}

		// Sleep well past the window boundary (3x) so CI scheduling jitter
		// cannot make this rolling-window check flaky.
		time.Sleep(3 * limits.UserRateWindow)
		_, err = admission.CreateSubmission(ctx, submissionParams(userID, "rate-after-window"), idempotency(keyBase+"-after", 0, submissionParams(userID, "rate-after-window")))
		if err != nil {
			t.Fatalf("create after rolling window failed: %v", err)
		}
	})

	t.Run("per-user outstanding limit counts submissions and code runs", func(t *testing.T) {
		outstandingLimits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   2,
			GlobalOutstandingLimit: 1000,
			LockTimeout:            5 * time.Second,
		}
		outstandingAdmission := postgres.NewAdmissionControl(db, outstandingLimits)
		userID := admissionTestUserB
		keyBase := "outstanding-" + t.Name()
		_, err := outstandingAdmission.CreateSubmission(ctx, submissionParams(userID, "s1"), idempotency(keyBase+"-s1", 0, submissionParams(userID, "s1")))
		if err != nil {
			t.Fatalf("first submission failed: %v", err)
		}
		r1Params, r1ID := reserveCodeRunParams(ctx, t, outstandingAdmission, userID, "r1")
		_, err = outstandingAdmission.CreateCodeRun(ctx, r1Params, idempotency(keyBase+"-r1", 0, r1Params), 1000, r1ID)
		if err != nil {
			t.Fatalf("first code run failed: %v", err)
		}
		r2Params, r2ID := reserveCodeRunParams(ctx, t, outstandingAdmission, userID, "r2")
		_, err = outstandingAdmission.CreateCodeRun(ctx, r2Params, idempotency(keyBase+"-r2", 0, r2Params), 1000, r2ID)
		if !errors.Is(err, apperror.ErrRateLimit) {
			t.Fatalf("expected user outstanding limit, got %v", err)
		}

		// Resolve one pending record; the next create should succeed.
		if _, err := db.Exec(ctx, "UPDATE code_runs SET status = 'ac' WHERE user_id = $1::uuid AND stdin = 'r1'", userID); err != nil {
			t.Fatalf("resolve code run: %v", err)
		}
		r3Params, r3ID := reserveCodeRunParams(ctx, t, outstandingAdmission, userID, "r3")
		_, err = outstandingAdmission.CreateCodeRun(ctx, r3Params, idempotency(keyBase+"-r3", 0, r3Params), 1000, r3ID)
		if err != nil {
			t.Fatalf("create after resolving one failed: %v", err)
		}
	})

	t.Run("global outstanding limit returns 503", func(t *testing.T) {
		markAllExecutionsTerminal(t, ctx, db)
		limits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   100,
			GlobalOutstandingLimit: 1,
			LockTimeout:            5 * time.Second,
		}
		globalAdmission := postgres.NewAdmissionControl(db, limits)
		_, err := globalAdmission.CreateSubmission(ctx, submissionParams(admissionTestUserA, "global-1"), nil)
		if err != nil {
			t.Fatalf("first global create failed: %v", err)
		}
		_, err = globalAdmission.CreateSubmission(ctx, submissionParams(admissionTestUserB, "global-2"), nil)
		if !errors.Is(err, apperror.ErrOverCapacity) {
			t.Fatalf("expected global over capacity, got %v", err)
		}
		appErr := err.(*apperror.AppError)
		if appErr.Code != "GLOBAL_OUTSTANDING_LIMIT_EXCEEDED" {
			t.Fatalf("error code = %q, want GLOBAL_OUTSTANDING_LIMIT_EXCEEDED", appErr.Code)
		}
		if appErr.RetryAfter <= 0 {
			t.Fatalf("Retry-After = %d", appErr.RetryAfter)
		}
	})

	t.Run("rejected calls leave no execution or outbox", func(t *testing.T) {
		markAllExecutionsTerminal(t, ctx, db)
		limits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   1,
			GlobalOutstandingLimit: 100,
			LockTimeout:            5 * time.Second,
		}
		rejectAdmission := postgres.NewAdmissionControl(db, limits)
		userID := admissionTestUserC
		onlyParams, onlyID := reserveCodeRunParams(ctx, t, rejectAdmission, userID, "only")
		_, err := rejectAdmission.CreateCodeRun(ctx, onlyParams, nil, 1000, onlyID)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}
		rejectedParams, rejectedID := reserveCodeRunParams(ctx, t, rejectAdmission, userID, "rejected")
		_, err = rejectAdmission.CreateCodeRun(ctx, rejectedParams, nil, 1000, rejectedID)
		if !errors.Is(err, apperror.ErrRateLimit) {
			t.Fatalf("expected rejection, got %v", err)
		}
		var count int64
		if err := db.QueryRow(ctx, "SELECT count(*) FROM code_runs WHERE user_id = $1::uuid AND stdin = 'rejected'", userID).Scan(&count); err != nil {
			t.Fatalf("count rejected code runs: %v", err)
		}
		if count != 0 {
			t.Fatalf("rejected code run left a row: %d", count)
		}
		if err := db.QueryRow(ctx, "SELECT count(*) FROM code_run_job_outbox WHERE code_run_id IN (SELECT id FROM code_runs WHERE user_id = $1::uuid AND stdin = 'rejected')", userID).Scan(&count); err != nil {
			t.Fatalf("count rejected outbox: %v", err)
		}
		if count != 0 {
			t.Fatalf("rejected code run left an outbox row: %d", count)
		}
	})
}

func TestAdmissionIdempotencyPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	t.Run("identical replay returns same record without new outbox", func(t *testing.T) {
		userID := admissionTestUserA
		params := submissionParams(userID, "replay")
		key := idempotency("replay-key", 0, params)
		first, err := admission.CreateSubmission(ctx, params, key)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}
		second, err := admission.CreateSubmission(ctx, params, key)
		if err != nil {
			t.Fatalf("replay failed: %v", err)
		}
		if first.ID != second.ID {
			t.Fatalf("replay returned different id: first=%d second=%d", first.ID, second.ID)
		}
		var count int64
		err = db.QueryRow(ctx, "SELECT count(*) FROM submissions WHERE user_id = $1::uuid AND source_code_key LIKE '%replay%'", userID).Scan(&count)
		if err != nil {
			t.Fatalf("count submissions: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected one submission, got %d", count)
		}
	})

	t.Run("changed payload returns 409", func(t *testing.T) {
		userID := admissionTestUserA
		params1 := submissionParams(userID, "conflict1")
		key := idempotency("conflict-key", 0, params1)
		_, err := admission.CreateSubmission(ctx, params1, key)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}
		params2 := submissionParams(userID, "conflict2")
		key2 := idempotency("conflict-key", 0, params2)
		_, err = admission.CreateSubmission(ctx, params2, key2)
		if !errors.Is(err, apperror.ErrConflict) {
			t.Fatalf("expected conflict, got %v", err)
		}
	})

	t.Run("concurrent same-key calls cannot duplicate", func(t *testing.T) {
		userID := admissionTestUserA
		params := submissionParams(userID, "concurrent")
		key := idempotency("concurrent-key", 0, params)
		var ids sync.Map
		var wg sync.WaitGroup
		errorsCh := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, err := admission.CreateSubmission(ctx, params, key)
				if err != nil {
					errorsCh <- err
					return
				}
				ids.Store(s.ID, struct{}{})
			}()
		}
		wg.Wait()
		close(errorsCh)
		for err := range errorsCh {
			if !errors.Is(err, apperror.ErrConflict) {
				t.Fatalf("unexpected concurrent error: %v", err)
			}
		}
		distinct := 0
		ids.Range(func(_, _ any) bool {
			distinct++
			return true
		})
		if distinct != 1 {
			t.Fatalf("expected exactly one distinct id, got %d", distinct)
		}
	})

	t.Run("replay works when admission limits are full", func(t *testing.T) {
		userID := admissionTestUserB
		limits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   1,
			GlobalOutstandingLimit: 100,
			LockTimeout:            5 * time.Second,
		}
		fullAdmission := postgres.NewAdmissionControl(db, limits)
		params := submissionParams(userID, "full")
		key := idempotency("full-key", 0, params)
		first, err := fullAdmission.CreateSubmission(ctx, params, key)
		if err != nil {
			t.Fatalf("first create failed: %v", err)
		}
		// A new request with a different key should be rejected because the
		// user already has one pending execution.
		_, err = fullAdmission.CreateSubmission(ctx, submissionParams(userID, "new"), idempotency("new-key", 0, submissionParams(userID, "new")))
		if !errors.Is(err, apperror.ErrRateLimit) {
			t.Fatalf("expected outstanding rejection, got %v", err)
		}
		// Replaying the original key must succeed and return the same record.
		replay, err := fullAdmission.CreateSubmission(ctx, params, key)
		if err != nil {
			t.Fatalf("replay under full limits failed: %v", err)
		}
		if replay.ID != first.ID {
			t.Fatalf("replay returned id %d, want %d", replay.ID, first.ID)
		}
	})
}

func TestCodeRunOutboxPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	dispatcher := &recordingDispatcher{}
	publisher := postgres.NewCodeRunOutboxPublisher(db, dispatcher, nil)

	userID := admissionTestUserA
	params, codeRunID := reserveCodeRunParams(ctx, t, admission, userID, "outbox")
	codeRun, err := admission.CreateCodeRun(ctx, params, nil, 1500, codeRunID)
	if err != nil {
		t.Fatalf("create code run failed: %v", err)
	}

	published, err := publisher.PublishPending(ctx, 1)
	if err != nil {
		t.Fatalf("publish pending failed: %v", err)
	}
	if published != 1 {
		t.Fatalf("published = %d, want 1", published)
	}
	if dispatcher.count.Load() != 1 {
		t.Fatalf("dispatch count = %d, want 1", dispatcher.count.Load())
	}
	job := dispatcher.lastJob.Load().(repository.ExecutionJob)
	if job.Reference().Kind() != repository.ExecutionReferenceCodeRun || job.Reference().ID() != fmt.Sprintf("%d", codeRun.ID) {
		t.Fatalf("reference = %s/%s", job.Reference().Kind(), job.Reference().ID())
	}
	stdin, ok := job.Mode().Stdin()
	if !ok || stdin.Stdin != "outbox" || stdin.TimeLimitMS != 1500 {
		t.Fatalf("stdin execution = %+v", stdin)
	}
	if job.SourceCodeKey() != fmt.Sprintf("code-runs/%s/%d/main.py", userID, codeRun.ID) {
		t.Fatalf("source key = %q", job.SourceCodeKey())
	}

	// Second publish should find nothing because dispatched_at is set.
	published, err = publisher.PublishPending(ctx, 1)
	if err != nil {
		t.Fatalf("second publish failed: %v", err)
	}
	if published != 0 {
		t.Fatalf("published = %d after dispatch, want 0", published)
	}
}

func TestCodeRunOutboxRetryPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	failingDispatcher := &failNTimesDispatcher{failuresRemaining: 2}
	publisher := postgres.NewCodeRunOutboxPublisher(db, failingDispatcher, nil)

	userID := admissionTestUserA
	retryParams, retryID := reserveCodeRunParams(ctx, t, admission, userID, "retry")
	_, err := admission.CreateCodeRun(ctx, retryParams, nil, 1000, retryID)
	if err != nil {
		t.Fatalf("create code run failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := publisher.PublishPending(ctx, 1)
		if i < 2 {
			if err == nil {
				t.Fatalf("iteration %d expected dispatch error", i)
			}
			// Force the record to be immediately reclaimable for the test.
			if _, updateErr := db.Exec(ctx, "UPDATE code_run_job_outbox SET available_at = now()"); updateErr != nil {
				t.Fatalf("reset available_at: %v", updateErr)
			}
		} else {
			if err != nil {
				t.Fatalf("iteration %d expected success, got %v", i, err)
			}
		}
	}
	if failingDispatcher.successCount.Load() != 1 {
		t.Fatalf("success count = %d, want 1", failingDispatcher.successCount.Load())
	}
	if failingDispatcher.failureCount.Load() != 2 {
		t.Fatalf("failure count = %d, want 2", failingDispatcher.failureCount.Load())
	}

	var attempts int
	var dispatchedAt *time.Time
	if err := db.QueryRow(ctx, "SELECT attempts, dispatched_at FROM code_run_job_outbox").Scan(&attempts, &dispatchedAt); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if dispatchedAt == nil {
		t.Fatal("dispatched_at is nil after success")
	}
}

func TestAdmissionHTTPPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          1,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	authMw := middleware.NewAuthMiddleware(nil, true)
	codeRunHandler := handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
		postgres.NewCodeRunRepository(db),
		&fakeAdmissionSourceCodeStorage{},
		admission,
		1000,
		255,
	))
	r := chi.NewRouter()
	r.Use(authMw.InjectActor)
	r.With(authMw.RequireAuth).Post("/api/v1/code-runs", codeRunHandler.Create)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", strings.NewReader(`{"language":"python","source_code":"print(1)"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-User-ID", admissionTestUserA)
	req1.Header.Set("X-User-Role", "user")
	res1 := httptest.NewRecorder()
	r.ServeHTTP(res1, req1)
	if res1.Code != http.StatusCreated {
		t.Fatalf("first create returned %d: %s", res1.Code, res1.Body)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", strings.NewReader(`{"language":"python","source_code":"print(2)"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-User-ID", admissionTestUserA)
	req2.Header.Set("X-User-Role", "user")
	res2 := httptest.NewRecorder()
	r.ServeHTTP(res2, req2)
	if res2.Code != http.StatusTooManyRequests {
		t.Fatalf("second create returned %d, want 429: %s", res2.Code, res2.Body)
	}
	retryAfter := res2.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("missing Retry-After header")
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res2.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != "USER_RATE_LIMIT_EXCEEDED" {
		t.Fatalf("error code = %q", body.Error.Code)
	}
}

func TestAdmissionConcurrentSlotsPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	t.Run("exactly one per-user outstanding slot", func(t *testing.T) {
		limits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   1,
			GlobalOutstandingLimit: 100,
			LockTimeout:            5 * time.Second,
		}
		admission := postgres.NewAdmissionControl(db, limits)
		userID := admissionTestUserA

		var wg sync.WaitGroup
		successes := atomic.Int32{}
		errorsCh := make(chan error, 16)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id, err := admission.ReserveCodeRunID(ctx)
				if err != nil {
					errorsCh <- fmt.Errorf("reserve code run id: %w", err)
					return
				}
				params := codeRunParams(userID, fmt.Sprintf("concurrent-user-%d", i), id)
				_, err = admission.CreateCodeRun(ctx, params, nil, 1000, id)
				if err == nil {
					successes.Add(1)
					return
				}
				errorsCh <- err
			}(i)
		}
		wg.Wait()
		close(errorsCh)

		if successes.Load() != 1 {
			t.Fatalf("expected exactly one success, got %d", successes.Load())
		}
		for err := range errorsCh {
			if !errors.Is(err, apperror.ErrRateLimit) && !errors.Is(err, apperror.ErrOverCapacity) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		var runCount, outboxCount int64
		markerPrefix := "concurrent-user-"
		if err := db.QueryRow(ctx, "SELECT count(*) FROM code_runs WHERE user_id = $1::uuid AND stdin LIKE $2", userID, markerPrefix+"%").Scan(&runCount); err != nil {
			t.Fatalf("count code runs: %v", err)
		}
		if runCount != 1 {
			t.Fatalf("expected one code run row, got %d", runCount)
		}
		if err := db.QueryRow(ctx, `SELECT count(*) FROM code_run_job_outbox o JOIN code_runs r ON r.id = o.code_run_id WHERE r.user_id = $1::uuid AND r.stdin LIKE $2`, userID, markerPrefix+"%").Scan(&outboxCount); err != nil {
			t.Fatalf("count outbox: %v", err)
		}
		if outboxCount != 1 {
			t.Fatalf("expected one outbox row, got %d", outboxCount)
		}
	})

	t.Run("exactly one global outstanding slot across different users", func(t *testing.T) {
		markAllExecutionsTerminal(t, ctx, db)
		limits := postgres.AdmissionLimits{
			UserRateLimit:          100,
			UserRateWindow:         10 * time.Second,
			UserOutstandingLimit:   100,
			GlobalOutstandingLimit: 1,
			LockTimeout:            5 * time.Second,
		}
		admission := postgres.NewAdmissionControl(db, limits)

		var wg sync.WaitGroup
		successes := atomic.Int32{}
		errorsCh := make(chan error, 16)
		users := []string{admissionTestUserA, admissionTestUserB, admissionTestUserC}
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				userID := users[i%len(users)]
				id, err := admission.ReserveCodeRunID(ctx)
				if err != nil {
					errorsCh <- fmt.Errorf("reserve code run id: %w", err)
					return
				}
				params := codeRunParams(userID, fmt.Sprintf("concurrent-global-%d", i), id)
				_, err = admission.CreateCodeRun(ctx, params, nil, 1000, id)
				if err == nil {
					successes.Add(1)
					return
				}
				errorsCh <- err
			}(i)
		}
		wg.Wait()
		close(errorsCh)

		if successes.Load() != 1 {
			t.Fatalf("expected exactly one success, got %d", successes.Load())
		}
		for err := range errorsCh {
			if !errors.Is(err, apperror.ErrOverCapacity) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		var runCount, outboxCount int64
		markerPrefix := "concurrent-global-"
		if err := db.QueryRow(ctx, "SELECT count(*) FROM code_runs WHERE stdin LIKE $1", markerPrefix+"%").Scan(&runCount); err != nil {
			t.Fatalf("count code runs: %v", err)
		}
		if runCount != 1 {
			t.Fatalf("expected one code run row, got %d", runCount)
		}
		if err := db.QueryRow(ctx, `SELECT count(*) FROM code_run_job_outbox o JOIN code_runs r ON r.id = o.code_run_id WHERE r.stdin LIKE $1`, markerPrefix+"%").Scan(&outboxCount); err != nil {
			t.Fatalf("count outbox: %v", err)
		}
		if outboxCount != 1 {
			t.Fatalf("expected one outbox row, got %d", outboxCount)
		}
	})
}

func TestAdmissionRollbackPostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	// Inject a trigger that forces the admission transaction to rollback after
	// the idempotency key has been written, so we can assert all three tables
	// (execution, outbox, idempotency) are left empty.
	if _, err := db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION force_idempotency_rollback() RETURNS trigger AS $$
		BEGIN
			IF NEW.key LIKE '%rollback%' THEN
				RAISE EXCEPTION 'injected rollback after idempotency';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER trg_force_idempotency_rollback
			AFTER INSERT ON execution_idempotency_keys
			FOR EACH ROW EXECUTE FUNCTION force_idempotency_rollback();
	`); err != nil {
		t.Fatalf("install rollback trigger: %v", err)
	}

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)
	userID := admissionTestUserA
	params, id := reserveCodeRunParams(ctx, t, admission, userID, "rollback")
	key := idempotency("rollback-key", 0, params)

	_, err := admission.CreateCodeRun(ctx, params, key, 1000, id)
	if err == nil {
		t.Fatal("expected injected rollback error")
	}

	var runCount, outboxCount, idemCount int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM code_runs WHERE user_id = $1::uuid AND stdin = 'rollback'", userID).Scan(&runCount); err != nil {
		t.Fatalf("count code runs: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("expected no code run after rollback, got %d", runCount)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM code_run_job_outbox").Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("expected no outbox after rollback, got %d", outboxCount)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM execution_idempotency_keys WHERE key = $1", key.Key).Scan(&idemCount); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idemCount != 0 {
		t.Fatalf("expected no idempotency key after rollback, got %d", idemCount)
	}
}

func TestCodeRunIdempotencyCascadePostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)
	userID := admissionTestUserA

	subParams := submissionParams(userID, "cascade-sub")
	subKey := idempotency("cascade-sub-key", 0, subParams)
	sub, err := admission.CreateSubmission(ctx, subParams, subKey)
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}

	runParams, runID := reserveCodeRunParams(ctx, t, admission, userID, "cascade-run")
	runKey := idempotency("cascade-run-key", 0, runParams)
	run, err := admission.CreateCodeRun(ctx, runParams, runKey, 1000, runID)
	if err != nil {
		t.Fatalf("create code run: %v", err)
	}

	if _, err := db.Exec(ctx, "DELETE FROM submissions WHERE id = $1", sub.ID); err != nil {
		t.Fatalf("delete submission: %v", err)
	}
	if _, err := db.Exec(ctx, "DELETE FROM code_runs WHERE id = $1", run.ID); err != nil {
		t.Fatalf("delete code run: %v", err)
	}

	var subIdemCount, runIdemCount int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM execution_idempotency_keys WHERE submission_id = $1", sub.ID).Scan(&subIdemCount); err != nil {
		t.Fatalf("count submission idempotency: %v", err)
	}
	if subIdemCount != 0 {
		t.Fatalf("submission idempotency key was not cascaded: %d", subIdemCount)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM execution_idempotency_keys WHERE code_run_id = $1", run.ID).Scan(&runIdemCount); err != nil {
		t.Fatalf("count code run idempotency: %v", err)
	}
	if runIdemCount != 0 {
		t.Fatalf("code run idempotency key was not cascaded: %d", runIdemCount)
	}
}

func TestCodeRunPrestoredKeyAndStorageFailurePostgres(t *testing.T) {
	databaseURL := os.Getenv("ADMISSION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set ADMISSION_TEST_DATABASE_URL to run the PostgreSQL admission integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := admissionTestDatabase(t, ctx, databaseURL)
	seedAdmissionTestData(t, ctx, db)

	limits := postgres.AdmissionLimits{
		UserRateLimit:          100,
		UserRateWindow:         10 * time.Second,
		UserOutstandingLimit:   100,
		GlobalOutstandingLimit: 1000,
		LockTimeout:            5 * time.Second,
	}
	admission := postgres.NewAdmissionControl(db, limits)

	authMw := middleware.NewAuthMiddleware(nil, true)
	failingStorage := &conditionalFailingSourceCodeStorage{failNext: true}
	codeRunHandler := handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
		postgres.NewCodeRunRepository(db),
		failingStorage,
		admission,
		1000,
		255,
	))
	r := chi.NewRouter()
	r.Use(authMw.InjectActor)
	r.With(authMw.RequireAuth).Post("/api/v1/code-runs", codeRunHandler.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", strings.NewReader(`{"language":"python","source_code":"print(1)"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", admissionTestUserA)
	req.Header.Set("X-User-Role", "user")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on storage failure, got %d: %s", res.Code, res.Body)
	}

	var runCount, outboxCount int64
	if err := db.QueryRow(ctx, "SELECT count(*) FROM code_runs WHERE user_id = $1::uuid", admissionTestUserA).Scan(&runCount); err != nil {
		t.Fatalf("count code runs: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("storage failure created code run row: %d", runCount)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM code_run_job_outbox").Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("storage failure created outbox row: %d", outboxCount)
	}

	// Retry with a working storage should succeed and commit the pre-stored key.
	workingStorage := &conditionalFailingSourceCodeStorage{}
	codeRunHandler = handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
		postgres.NewCodeRunRepository(db),
		workingStorage,
		admission,
		1000,
		255,
	))
	r = chi.NewRouter()
	r.Use(authMw.InjectActor)
	r.With(authMw.RequireAuth).Post("/api/v1/code-runs", codeRunHandler.Create)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", strings.NewReader(`{"language":"python","source_code":"print(1)"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-User-ID", admissionTestUserA)
	req2.Header.Set("X-User-Role", "user")
	res2 := httptest.NewRecorder()
	r.ServeHTTP(res2, req2)
	if res2.Code != http.StatusCreated {
		t.Fatalf("retry returned %d: %s", res2.Code, res2.Body)
	}

	var storedKey string
	if err := db.QueryRow(ctx, `SELECT payload->>'sourceCodeKey' FROM code_run_job_outbox`).Scan(&storedKey); err != nil {
		t.Fatalf("read outbox payload: %v", err)
	}
	if storedKey == "" {
		t.Fatal("outbox payload missing sourceCodeKey")
	}
	if storedKey != workingStorage.lastKey {
		t.Fatalf("outbox source key %q does not match pre-stored key %q", storedKey, workingStorage.lastKey)
	}
}

const (
	admissionTestUserA = "00000000-0000-4000-8000-000000000001"
	admissionTestUserB = "00000000-0000-4000-8000-000000000002"
	admissionTestUserC = "00000000-0000-4000-8000-000000000003"
)

func admissionTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	schema := pgx.Identifier{fmt.Sprintf("admission_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("clean up test schema: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	migrations, err := filepath.Glob("../../../db/migrations/*.up.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("find migrations: %v (found %d)", err, len(migrations))
	}
	for _, path := range migrations {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", path, err)
		}
	}
	return db
}

func seedAdmissionTestData(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	_, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email) VALUES
	('00000000-0000-4000-8000-000000000001', 'test', 'a', 'A', 'a@example.test'),
	('00000000-0000-4000-8000-000000000002', 'test', 'b', 'B', 'b@example.test'),
	('00000000-0000-4000-8000-000000000003', 'test', 'c', 'C', 'c@example.test');
INSERT INTO problems (id, title, statement, time_limit_ms, memory_limit_kb, created_by)
VALUES (1, 'Test', 'Test', 1000, 262144, '00000000-0000-4000-8000-000000000001');
INSERT INTO test_cases (id, problem_id, input, expected_output) VALUES (1, 1, '1 2', '3');
`)
	if err != nil {
		t.Fatalf("seed test database: %v", err)
	}
}

func markAllExecutionsTerminal(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	if _, err := db.Exec(ctx, "UPDATE submissions SET status = 'ac'; UPDATE code_runs SET status = 'ac';"); err != nil {
		t.Fatalf("mark executions terminal: %v", err)
	}
}

func submissionParams(userID, marker string) repository.CreateSubmissionWithOutboxParams {
	return repository.CreateSubmissionWithOutboxParams{
		CreateSubmissionParams: repository.CreateSubmissionParams{
			UserID:        userID,
			ProblemID:     1,
			Language:      domain.SubmissionLanguagePython,
			SourceCodeKey: fmt.Sprintf("submissions/%s/%s/main.py", userID, marker),
			TestCasesKey:  "problems/1/testcases/current.json",
			Status:        domain.SubmissionStatusPending,
			Score:         0,
		},
		TimeLimitMS: 1000,
	}
}

func codeRunParams(userID, marker string, codeRunID int64) repository.CreateCodeRunParams {
	return repository.CreateCodeRunParams{
		UserID:        userID,
		Language:      domain.ExecutionLanguagePython,
		SourceCode:    fmt.Sprintf("print('%s')", marker),
		Stdin:         marker,
		SourceCodeKey: codeRunSourceKey(userID, codeRunID),
	}
}

func codeRunSourceKey(userID string, codeRunID int64) string {
	return fmt.Sprintf("code-runs/%s/%d/main.py", userID, codeRunID)
}

func reserveCodeRunParams(ctx context.Context, t *testing.T, admission *postgres.AdmissionControl, userID, marker string) (repository.CreateCodeRunParams, int64) {
	t.Helper()
	id, err := admission.ReserveCodeRunID(ctx)
	if err != nil {
		t.Fatalf("reserve code run id: %v", err)
	}
	return codeRunParams(userID, marker, id), id
}

func idempotency(key string, seq int, params any) *repository.IdempotencyKeyInfo {
	var fp string
	switch p := params.(type) {
	case repository.CreateSubmissionWithOutboxParams:
		fp = canonicalTestFingerprint(map[string]any{
			"problem_id":  p.ProblemID,
			"language":    string(p.Language),
			"source_code": p.SourceCodeKey,
		})
	case repository.CreateCodeRunParams:
		fp = canonicalTestFingerprint(map[string]any{
			"language":    string(p.Language),
			"source_code": p.SourceCode,
			"stdin":       p.Stdin,
		})
	}
	return &repository.IdempotencyKeyInfo{
		Namespace:   "test",
		Key:         fmt.Sprintf("%s-%d", key, seq),
		Fingerprint: fp,
	}
}

func canonicalTestFingerprint(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

type recordingDispatcher struct {
	count   atomic.Int32
	lastJob atomic.Value
}

func (d *recordingDispatcher) Dispatch(_ context.Context, job repository.ExecutionJob) error {
	d.count.Add(1)
	d.lastJob.Store(job)
	return nil
}

type failNTimesDispatcher struct {
	failuresRemaining int
	failureCount      atomic.Int32
	successCount      atomic.Int32
	mu                sync.Mutex
}

func (d *failNTimesDispatcher) Dispatch(_ context.Context, _ repository.ExecutionJob) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failuresRemaining > 0 {
		d.failuresRemaining--
		d.failureCount.Add(1)
		return errors.New("dispatch failure")
	}
	d.successCount.Add(1)
	return nil
}

type fakeAdmissionSourceCodeStorage struct{}

func (s *fakeAdmissionSourceCodeStorage) PutSourceCode(_ context.Context, _ repository.PutSourceCodeParams) error {
	return nil
}

type conditionalFailingSourceCodeStorage struct {
	failNext bool
	lastKey  string
}

func (s *conditionalFailingSourceCodeStorage) PutSourceCode(_ context.Context, params repository.PutSourceCodeParams) error {
	if s.failNext {
		return errors.New("injected storage failure")
	}
	s.lastKey = params.Key
	return nil
}
