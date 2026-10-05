package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

// Uses an isolated schema and the real migrations, repository, usecase, and HTTP
// handler. Set SUBMISSION_DETAILS_TEST_DATABASE_URL to a local test database.
func TestSubmissionDetailsPostgres(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('00000000-0000-4000-8000-000000000001', 'test', 'details', 'Test', 'details@example.test');
INSERT INTO problems (id, title, statement, time_limit_ms, memory_limit_kb, created_by)
VALUES (42, 'Test', 'Test', 1000, 262144, '00000000-0000-4000-8000-000000000001');
INSERT INTO test_cases (id, problem_id, input, expected_output)
VALUES (10, 42, 'secret-input', 'secret-expected'), (12, 42, 'secret-input', 'secret-expected'),
       (15, 42, 'not-executed', 'not-executed');
INSERT INTO submissions (id, user_id, problem_id, language, source_code_key, test_cases_key, status, score, judged_at)
VALUES (7, '00000000-0000-4000-8000-000000000001', 42, 'python', 'source.py', 'cases.json', 'wa', 1, now()),
       (8, '00000000-0000-4000-8000-000000000001', 42, 'python', 'other.py', 'cases.json', 'ac', 3, now());
INSERT INTO submission_results (submission_id, test_case_id, status, execution_time_ms, memory_kb, stdout, stderr)
VALUES (7, 12, 'wa', NULL, NULL, 'secret-stdout', 'secret-stderr'),
       (8, 10, 'ac', 999, 999, 'other submission', ''),
       (7, 10, 'ac', 0, 1024, 'secret-stdout', 'secret-stderr');
`)
	if err != nil {
		t.Fatalf("seed test database: %v", err)
	}

	repo := postgres.NewSubmissionRepository(db)
	h := handler.NewSubmissionHandler(usecase.NewSubmissionUsecase(repo, nil, nil))
	auth := middleware.NewAuthMiddleware(nil, true)
	router := chi.NewRouter()
	router.Use(auth.InjectActor)
	router.With(auth.RequireAuth).Get("/api/v1/submissions/{submission_id}", h.Get)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions/7", nil)
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", "user")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET returned %d: %s", res.Code, res.Body)
	}
	var body struct {
		Data struct {
			Status string `json:"status"`
			Cases  []struct {
				ID      int64  `json:"test_case_id"`
				Status  string `json:"status"`
				Runtime *int   `json:"execution_time_ms"`
				Memory  *int   `json:"memory_kb"`
			} `json:"case_results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cases := body.Data.Cases
	if body.Data.Status != "wa" || len(cases) != 2 || cases[0].ID != 10 || cases[1].ID != 12 {
		t.Fatalf("results must be scoped to submission 7, sorted, and omit unexecuted cases: %s", res.Body)
	}
	if cases[0].Status != "ac" || cases[0].Runtime == nil || *cases[0].Runtime != 0 || cases[0].Memory == nil || *cases[0].Memory != 1024 {
		t.Fatalf("recorded zero and non-null metrics were not preserved: %+v", cases[0])
	}
	if cases[1].Status != "wa" || cases[1].Runtime != nil || cases[1].Memory != nil {
		t.Fatalf("missing metrics must remain null: %+v", cases[1])
	}
	for _, forbidden := range []string{"secret-", "stdout", "stderr", "expected_output", `"input"`} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("response exposes hidden test data %q: %s", forbidden, res.Body)
		}
	}
	results, err := repo.ListCaseResults(ctx, 999)
	if err != nil || results == nil || len(results) != 0 {
		t.Fatalf("empty query = %+v, %v; want empty non-nil results", results, err)
	}
}

func submissionDetailsTestDatabase(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	schema := pgx.Identifier{fmt.Sprintf("submission_details_%d", time.Now().UnixNano())}.Sanitize()
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
