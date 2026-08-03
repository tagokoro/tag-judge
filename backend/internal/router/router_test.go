package router

import (
	"bytes"
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

func TestCreateSubmissionRoute(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems/1/submissions", strings.NewReader(`{"language":"python","source_code":"print(1)"}`))
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	var body struct {
		Data struct {
			SourceCodeKey string `json:"source_code_key"`
			Status        string `json:"status"`
			Score         int    `json:"score"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.SourceCodeKey == "" || body.Data.Status != "pending" || body.Data.Score != 0 {
		t.Fatalf("submission = %+v", body.Data)
	}
}

func TestCreateSubmissionUnauthorized(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems/1/submissions", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestCreateSubmissionLanguage(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems/1/submissions", strings.NewReader(`{"language":"go","source_code":"package main"}`))
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, res, "INVALID_LANGUAGE")
}

func TestGetSubmissionID(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions/not-an-id", nil)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, res, "INVALID_SUBMISSION_ID")
}

func TestListSubmissionsUnauthorized(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestListSubmissionsLimit(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/submissions?limit=0", nil)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, res, "INVALID_LIMIT")
}

func TestCodeRunsAuth(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/code-runs", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestCodeRunsCreate(t *testing.T) {
	router := newTestRouter()
	body := bytes.NewBufferString(`{"language":"python","source_code":"print('ok')"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", body)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}

	var response struct {
		Data struct {
			UserID string `json:"user_id"`
			Stdin  string `json:"stdin"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.UserID != routerUserID || response.Data.Stdin != "" || response.Data.Status != "pending" {
		t.Fatalf("data = %+v", response.Data)
	}
}

func TestCodeRunsLanguage(t *testing.T) {
	router := newTestRouter()
	body := bytes.NewBufferString(`{"language":"go","source_code":"package main"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", body)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestCodeRunsJSON(t *testing.T) {
	router := newTestRouter()
	body := bytes.NewBufferString(`{"language":"python","source_code":"print(1)","extra":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/code-runs", body)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestCodeRunsForbidden(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/code-runs/1", nil)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestCodeRunsLimit(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/code-runs?limit=0", nil)
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestCodeRunsAdmin(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/code-runs/1", nil)
	setAuth(req, "admin")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestGroupsRequireAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/groups", body: `{"name":"Team"}`},
		{method: http.MethodGet, path: "/api/v1/groups"},
		{method: http.MethodGet, path: "/api/v1/groups/1"},
		{method: http.MethodPost, path: "/api/v1/groups/1/members", body: `{"user_id":"00000000-0000-4000-8000-000000000002","role":"member"}`},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, bytes.NewBufferString(route.body))
		res := httptest.NewRecorder()
		newTestRouter().ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", route.method, route.path, res.Code, http.StatusUnauthorized)
		}
	}
}

func TestCreateGroupRoute(t *testing.T) {
	repo := newRouterGroupRepository()
	router := newTestRouterWithGroups(repo)
	req := groupRequest(http.MethodPost, "/api/v1/groups", `{"name":"Team","description":"Practice"}`, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	if repo.createParams.Name != "Team" || repo.createParams.CreatedBy == "" {
		t.Fatalf("params = %+v", repo.createParams)
	}
}

func TestCreateGroupForbidden(t *testing.T) {
	router := newTestRouter()
	req := groupRequest(http.MethodPost, "/api/v1/groups", `{"name":"Team"}`, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestCreateGroupValidation(t *testing.T) {
	router := newTestRouter()
	for _, body := range []string{`{"name":""}`, `{"name":"Team","unknown":true}`, `{`, `{"name":"Team"} {}`} {
		req := groupRequest(http.MethodPost, "/api/v1/groups", body, "author")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("body = %q, status = %d, want %d", body, res.Code, http.StatusBadRequest)
		}
	}
}

func TestGetGroupRoute(t *testing.T) {
	router := newTestRouter()
	req := groupRequest(http.MethodGet, "/api/v1/groups/1", "", "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
}

func TestGetGroupForbidden(t *testing.T) {
	repo := newRouterGroupRepository()
	repo.member = nil
	router := newTestRouterWithGroups(repo)
	req := groupRequest(http.MethodGet, "/api/v1/groups/1", "", "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestGroupIDValidation(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{"/api/v1/groups/not-a-number", "/api/v1/groups/0"} {
		req := groupRequest(http.MethodGet, path, "", "admin")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("path = %q, status = %d, want %d", path, res.Code, http.StatusBadRequest)
		}
	}
}

func TestListGroupsRoute(t *testing.T) {
	router := newTestRouter()
	req := groupRequest(http.MethodGet, "/api/v1/groups", "", "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
}

func TestAddMemberRoute(t *testing.T) {
	repo := newRouterGroupRepository()
	router := newTestRouterWithGroups(repo)
	req := groupRequest(http.MethodPost, "/api/v1/groups/1/members", `{"user_id":"00000000-0000-4000-8000-000000000002","role":"member"}`, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	if repo.addParams.UserID != "00000000-0000-4000-8000-000000000002" {
		t.Fatalf("params = %+v", repo.addParams)
	}
}

func TestAddMemberForbidden(t *testing.T) {
	repo := newRouterGroupRepository()
	repo.member.Role = domain.GroupRoleMember
	router := newTestRouterWithGroups(repo)
	req := groupRequest(http.MethodPost, "/api/v1/groups/1/members", `{"user_id":"00000000-0000-4000-8000-000000000002","role":"member"}`, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestAddMemberValidation(t *testing.T) {
	router := newTestRouter()
	req := groupRequest(http.MethodPost, "/api/v1/groups/1/members", `{"user_id":"invalid","role":"manager"}`, "admin")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

const routerUserID = "00000000-0000-4000-8000-000000000001"

func setAuth(req *http.Request, role string) {
	req.Header.Set("X-User-ID", routerUserID)
	req.Header.Set("X-User-Role", role)
}

func assertErrorCode(t *testing.T, res *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != want {
		t.Fatalf("error code = %q, want %q", body.Error.Code, want)
	}
}

func newTestRouter() http.Handler {
	return newTestRouterWithGroups(newRouterGroupRepository())
}

func newTestRouterWithGroups(groups repository.GroupRepository) http.Handler {
	authRepo := &fakeAuthRepository{}
	authUsecase := usecase.NewAuthUsecase(authRepo, nil, 0)
	problemRepo := &fakeProblemRepository{}
	return New(Handlers{
		Problems: handler.NewProblemHandler(usecase.NewProblemUsecase(problemRepo)),
		CodeRuns: handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
			&fakeCodeRunRepository{},
			&fakeSourceCodeStorage{},
			&fakeExecutionDispatcher{},
			2500,
		)),
		Groups: handler.NewGroupHandler(usecase.NewGroupUsecase(groups)),
		Submissions: handler.NewSubmissionHandler(usecase.NewSubmissionUsecase(
			&fakeSubmissionRepository{},
			problemRepo,
			&fakeSourceCodeStorage{},
			&fakeExecutionDispatcher{},
		)),
		Me:       handler.NewMeHandler(),
		AuthHTTP: handler.NewAuthHandler(authUsecase, false, "/"),
		Auth:     appmiddleware.NewAuthMiddleware(authRepo, true),
	})
}

func groupRequest(method, path, body, role string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-User-Role", role)
	return req
}

type fakeProblemRepository struct{}

func (r *fakeProblemRepository) ListPublic(_ context.Context, _ repository.ListProblemsParams) ([]domain.Problem, error) {
	return []domain.Problem{}, nil
}

func (r *fakeProblemRepository) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	return &domain.Problem{ID: id, TimeLimitMS: 1000}, nil
}

type fakeCodeRunRepository struct{}

func (r *fakeCodeRunRepository) Create(_ context.Context, params repository.CreateCodeRunParams) (*domain.CodeRun, error) {
	return &domain.CodeRun{
		ID:         1,
		UserID:     params.UserID,
		Language:   params.Language,
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
		Status:     domain.CodeRunStatusPending,
	}, nil
}

func (r *fakeCodeRunRepository) FindByID(_ context.Context, id int64) (*domain.CodeRun, error) {
	return &domain.CodeRun{ID: id, UserID: "00000000-0000-4000-8000-000000000002"}, nil
}

func (r *fakeCodeRunRepository) ListByUserID(_ context.Context, _ repository.ListCodeRunsParams) ([]domain.CodeRun, error) {
	return []domain.CodeRun{}, nil
}

type fakeSubmissionRepository struct{}

func (r *fakeSubmissionRepository) Create(_ context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	return &domain.Submission{
		ID:            1,
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		Status:        params.Status,
		Score:         params.Score,
	}, nil
}

func (r *fakeSubmissionRepository) FindByID(_ context.Context, id int64) (*domain.Submission, error) {
	return &domain.Submission{ID: id, UserID: "00000000-0000-4000-8000-000000000001"}, nil
}

func (r *fakeSubmissionRepository) ListByUserID(_ context.Context, _ repository.ListSubmissionsParams) ([]domain.Submission, error) {
	return []domain.Submission{}, nil
}

type fakeSourceCodeStorage struct{}

func (s *fakeSourceCodeStorage) PutSourceCode(_ context.Context, _ repository.PutSourceCodeParams) error {
	return nil
}

type fakeExecutionDispatcher struct{}

func (d *fakeExecutionDispatcher) Dispatch(_ context.Context, _ repository.ExecutionJob) error {
	return nil
}

type routerGroupRepository struct {
	member       *domain.GroupMember
	createParams repository.CreateGroupParams
	addParams    repository.AddGroupMemberParams
}

func newRouterGroupRepository() *routerGroupRepository {
	return &routerGroupRepository{member: &domain.GroupMember{
		GroupID: 1,
		UserID:  "00000000-0000-4000-8000-000000000001",
		Role:    domain.GroupRoleOwner,
	}}
}

func (r *routerGroupRepository) CreateWithOwner(_ context.Context, params repository.CreateGroupParams) (*domain.Group, error) {
	r.createParams = params
	return &domain.Group{ID: 1, Name: params.Name, Description: params.Description, CreatedBy: params.CreatedBy}, nil
}

func (r *routerGroupRepository) FindByID(_ context.Context, id int64) (*domain.Group, error) {
	return &domain.Group{ID: id, Name: "Team"}, nil
}

func (r *routerGroupRepository) ListByUserID(_ context.Context, _ string) ([]domain.Group, error) {
	return []domain.Group{{ID: 1, Name: "Team"}}, nil
}

func (r *routerGroupRepository) FindMember(_ context.Context, _ int64, _ string) (*domain.GroupMember, error) {
	return r.member, nil
}

func (r *routerGroupRepository) AddMember(_ context.Context, params repository.AddGroupMemberParams) (*domain.GroupMember, error) {
	r.addParams = params
	return &domain.GroupMember{GroupID: params.GroupID, UserID: params.UserID, Role: params.Role}, nil
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
