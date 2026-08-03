package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/auth"
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

func TestMeRouteSessionCookie(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "tag_judge_session", Value: "valid-session"})
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
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

func TestCreateProblemRoute(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems", strings.NewReader(`{
		"title": "New Problem",
		"statement": "Solve it.",
		"time_limit_ms": 1000,
		"memory_limit_kb": 262144,
		"visibility": "public"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}
}

func TestCreateProblemRouteRequiresAuthor(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems", strings.NewReader(`{
		"title": "New Problem",
		"statement": "Solve it.",
		"time_limit_ms": 1000,
		"memory_limit_kb": 262144,
		"visibility": "public"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000003")
	req.Header.Set("X-User-Role", "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func newTestRouter() http.Handler {
	authRepo := &fakeAuthRepository{}
	authUsecase := usecase.NewAuthUsecase(authRepo, nil, 0)
	return New(Handlers{
		Problems: handler.NewProblemHandler(usecase.NewProblemUsecase(&fakeProblemRepository{})),
		Me:       handler.NewMeHandler(),
		AuthHTTP: handler.NewAuthHandler(authUsecase, false, "/"),
		Auth:     appmiddleware.NewAuthMiddleware(authRepo, true),
	})
}

type fakeProblemRepository struct{}

func (r *fakeProblemRepository) ListPublic(_ context.Context, _ repository.ListProblemsParams) ([]domain.Problem, error) {
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) ListVisible(_ context.Context, _ repository.ListProblemsParams, _ string, _ bool) ([]domain.Problem, error) {
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	return &domain.Problem{ID: id}, nil
}

func (r *fakeProblemRepository) FindVisibleByID(_ context.Context, id int64, actorUserID string, _ bool) (*domain.Problem, error) {
	return &domain.Problem{ID: id, CreatedBy: actorUserID}, nil
}

func (r *fakeProblemRepository) Create(_ context.Context, params repository.ProblemMutationParams) (*domain.Problem, error) {
	return &domain.Problem{
		ID:            1,
		Title:         params.Title,
		Statement:     params.Statement,
		TimeLimitMS:   params.TimeLimitMS,
		MemoryLimitKB: params.MemoryLimitKB,
		Visibility:    params.Visibility,
		CreatedBy:     params.CreatedBy,
	}, nil
}

func (r *fakeProblemRepository) Update(_ context.Context, id int64, params repository.ProblemMutationParams) (*domain.Problem, error) {
	return &domain.Problem{ID: id, Title: params.Title, CreatedBy: params.CreatedBy}, nil
}

func (r *fakeProblemRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeProblemRepository) ListTestCases(_ context.Context, problemID int64) ([]domain.TestCase, error) {
	return []domain.TestCase{{ID: 1, ProblemID: problemID, Score: 1}}, nil
}

func (r *fakeProblemRepository) CreateTestCase(_ context.Context, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: 1, ProblemID: params.ProblemID, Score: params.Score}, nil
}

func (r *fakeProblemRepository) UpdateTestCase(_ context.Context, id int64, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: id, ProblemID: params.ProblemID, Score: params.Score}, nil
}

func (r *fakeProblemRepository) DeleteTestCase(_ context.Context, _ int64, _ int64) error {
	return nil
}

func (r *fakeProblemRepository) CreateSubmission(_ context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	return &domain.Submission{ID: 1, UserID: params.UserID, ProblemID: params.ProblemID, Language: params.Language}, nil
}

func (r *fakeProblemRepository) ListSubmissionsByProblem(_ context.Context, problemID int64, userID string, _ bool) ([]domain.Submission, error) {
	return []domain.Submission{{ID: 1, UserID: userID, ProblemID: problemID}}, nil
}

func (r *fakeProblemRepository) FindSubmissionByID(_ context.Context, id int64) (*domain.Submission, error) {
	return &domain.Submission{ID: id, UserID: "00000000-0000-4000-8000-000000000001", ProblemID: 1}, nil
}

type fakeAuthRepository struct{}

func (r *fakeAuthRepository) UpsertGoogleUser(_ context.Context, _ repository.GoogleUser) (auth.Actor, error) {
	return auth.Actor{}, nil
}

func (r *fakeAuthRepository) CreateSession(_ context.Context, _ repository.CreateSessionParams) error {
	return nil
}

func (r *fakeAuthRepository) FindActorBySessionTokenHash(_ context.Context, _ string) (auth.Actor, error) {
	return auth.Actor{
		UserID: "00000000-0000-4000-8000-000000000001",
		Role:   domain.UserRoleAuthor,
	}, nil
}

func (r *fakeAuthRepository) RevokeSessionByTokenHash(_ context.Context, _ string) error {
	return nil
}
