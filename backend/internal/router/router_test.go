package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

func TestProblemsRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/problems", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestProblemsRouteEmptyLimit(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/problems?limit=", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestProblemsRouteZeroLimit(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/problems?limit=0", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "INVALID_LIMIT" {
		t.Fatalf("error code = %q, want INVALID_LIMIT", body.Error.Code)
	}
}

func TestCreateProblemRoute(t *testing.T) {
	router := newTestRouter()
	req := newCreateProblemRequest(domain.UserRoleAuthor, validProblemJSON())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	var body struct {
		Data struct {
			ID            int64  `json:"id"`
			TimeLimitMS   int    `json:"time_limit_ms"`
			MemoryLimitKB int    `json:"memory_limit_kb"`
			CreatedBy     string `json:"created_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID != 42 || body.Data.TimeLimitMS != 1000 || body.Data.MemoryLimitKB != 262144 {
		t.Fatalf("unexpected response: %+v", body.Data)
	}
	if body.Data.CreatedBy != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("created_by = %q", body.Data.CreatedBy)
	}
}

func TestCreateProblemUnauthorized(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems", bytes.NewBufferString(validProblemJSON()))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestCreateProblemForbidden(t *testing.T) {
	router := newTestRouter()
	req := newCreateProblemRequest(domain.UserRoleUser, validProblemJSON())
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestCreateProblemInvalid(t *testing.T) {
	router := newTestRouter()
	req := newCreateProblemRequest(domain.UserRoleAuthor, `{
		"title":"",
		"statement":"Compute $A+B$.",
		"time_limit_ms":1000,
		"memory_limit_kb":262144,
		"visibility":"private",
		"test_cases":[{"input":"","expected_output":"","is_sample":false}]
	}`)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "INVALID_TITLE" {
		t.Fatalf("error code = %q, want INVALID_TITLE", body.Error.Code)
	}
}

func TestCreateProblemInvalidCase(t *testing.T) {
	router := newTestRouter()
	req := newCreateProblemRequest(domain.UserRoleAuthor, `{
		"title":"A + B",
		"statement":"Compute $A+B$.",
		"time_limit_ms":1000,
		"memory_limit_kb":262144,
		"visibility":"private",
		"test_cases":[{"input":"","expected_output":""}]
	}`)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func newCreateProblemRequest(role domain.UserRole, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", string(role))
	return req
}

func validProblemJSON() string {
	return `{
		"title":"A + B",
		"statement":"Compute $A+B$.",
		"time_limit_ms":1000,
		"memory_limit_kb":262144,
		"visibility":"private",
		"test_cases":[
			{"input":"1 2\\n","expected_output":"3\\n","is_sample":true},
			{"input":"2 3\\n","expected_output":"5\\n","is_sample":false}
		]
	}`
}

func TestMeRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.UserID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("user_id = %q", body.Data.UserID)
	}
	if body.Data.Role != "author" {
		t.Fatalf("role = %q, want author", body.Data.Role)
	}
}

func TestMeRouteUnauthorized(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("error code = %q, want UNAUTHORIZED", body.Error.Code)
	}
}

func TestMeRouteInvalidAuthHeader(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", "invalid")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func newTestRouter() http.Handler {
	return New(Handlers{
		Problems: handler.NewProblemHandler(usecase.NewProblemUsecase(&fakeProblemRepository{})),
		Me:       handler.NewMeHandler(),
		Auth:     appmiddleware.NewAuthMiddleware(),
	})
}

type fakeProblemRepository struct{}

func (r *fakeProblemRepository) ListPublic(_ context.Context, _ repository.ListProblemsParams) ([]domain.Problem, error) {
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	return &domain.Problem{ID: id}, nil
}

func (r *fakeProblemRepository) Create(_ context.Context, params repository.CreateProblemParams) (*domain.Problem, error) {
	return &domain.Problem{
		ID:            42,
		Title:         params.Title,
		Statement:     params.Statement,
		TimeLimitMS:   params.TimeLimitMS,
		MemoryLimitKB: params.MemoryLimitKB,
		Visibility:    params.Visibility,
		CreatedBy:     params.CreatedBy,
	}, nil
}
