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
		"visibility":"private"
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/problems/42/test-cases", bytes.NewBufferString(`{"input":"","expected_output":""}`))
	setAuth(req, "author")
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
		"visibility":"private"
	}`
}

func TestTestCaseRoutes(t *testing.T) {
	router := newTestRouter()

	create := httptest.NewRequest(http.MethodPost, "/api/v1/problems/42/test-cases", bytes.NewBufferString(`{"input":"1 2\n","expected_output":"3\n","is_sample":true}`))
	setAuth(create, "author")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, create)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createRes.Code, createRes.Body.String())
	}

	update := httptest.NewRequest(http.MethodPatch, "/api/v1/problems/42/test-cases/7", bytes.NewBufferString(`{"is_sample":false}`))
	setAuth(update, "author")
	updateRes := httptest.NewRecorder()
	router.ServeHTTP(updateRes, update)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", updateRes.Code, updateRes.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/problems/42/test-cases/7", nil)
	setAuth(deleteRequest, "author")
	deleteRes := httptest.NewRecorder()
	router.ServeHTTP(deleteRes, deleteRequest)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", deleteRes.Code, deleteRes.Body.String())
	}
}

func TestProblemManageRoutes(t *testing.T) {
	router := newTestRouter()

	update := httptest.NewRequest(http.MethodPatch, "/api/v1/problems/42", bytes.NewBufferString(`{"title":"Updated"}`))
	setAuth(update, "author")
	updateRes := httptest.NewRecorder()
	router.ServeHTTP(updateRes, update)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", updateRes.Code, updateRes.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/problems/42", nil)
	setAuth(deleteRequest, "author")
	deleteRes := httptest.NewRecorder()
	router.ServeHTTP(deleteRes, deleteRequest)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", deleteRes.Code, deleteRes.Body.String())
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
	return newTestRouterWithRepositories(groups, &fakeSubmissionRepository{})
}

func newTestRouterWithRepositories(groups repository.GroupRepository, submissions repository.SubmissionRepository) http.Handler {
	authRepo := &fakeAuthRepository{}
	authUsecase := usecase.NewAuthUsecase(authRepo, nil, 0)
	problemRepo := &fakeProblemRepository{}
	testCaseRepo := &fakeRouterTestCaseRepository{}
	return New(Handlers{
		Problems:  handler.NewProblemHandler(usecase.NewProblemUsecase(problemRepo, testCaseRepo)),
		TestCases: handler.NewTestCaseHandler(usecase.NewTestCaseUsecase(problemRepo, testCaseRepo, &fakeSourceCodeStorage{})),
		CodeRuns: handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
			&fakeCodeRunRepository{},
			&fakeSourceCodeStorage{},
			&fakeExecutionDispatcher{},
			2500,
		)),
		Groups: handler.NewGroupHandler(usecase.NewGroupUsecase(groups)),
		Submissions: handler.NewSubmissionHandler(usecase.NewSubmissionUsecase(
			submissions,
			problemRepo,
			&fakeSourceCodeStorage{},
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
	return &domain.Problem{ID: id, TimeLimitMS: 1000, CurrentTestCasesKey: "problems/1/testcases/current.json", CreatedBy: routerUserID}, nil
}

func (r *fakeProblemRepository) FindByID(_ context.Context, id int64) (*domain.Problem, error) {
	return &domain.Problem{
		ID: id, Title: "A + B", Statement: "Compute.", TimeLimitMS: 1000, MemoryLimitKB: 262144,
		Visibility: domain.ProblemVisibilityPrivate, CreatedBy: routerUserID,
	}, nil
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

type fakeSubmissionRepository struct {
	submission       *domain.Submission
	findErr          error
	caseResults      []domain.SubmissionCaseResult
	caseResultsCalls int
	caseResultsErr   error
	list             []domain.Submission
}

func (r *fakeSubmissionRepository) Create(_ context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	return &domain.Submission{
		ID:            1,
		UserID:        params.UserID,
		ProblemID:     params.ProblemID,
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        params.Status,
		Score:         params.Score,
	}, nil
}

func (r *fakeSubmissionRepository) CreateWithOutbox(_ context.Context, params repository.CreateSubmissionWithOutboxParams) (*domain.Submission, error) {
	return r.Create(context.Background(), params.CreateSubmissionParams)
}

func (r *fakeSubmissionRepository) FindByID(_ context.Context, id int64) (*domain.Submission, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.submission != nil {
		return r.submission, nil
	}
	return &domain.Submission{ID: id, UserID: "00000000-0000-4000-8000-000000000001"}, nil
}

func (r *fakeSubmissionRepository) ListCaseResults(_ context.Context, _ int64) ([]domain.SubmissionCaseResult, error) {
	r.caseResultsCalls++
	return r.caseResults, r.caseResultsErr
}

func (r *fakeSubmissionRepository) ListByUserID(_ context.Context, _ repository.ListSubmissionsParams) ([]domain.Submission, error) {
	return r.list, nil
}

type fakeSourceCodeStorage struct{}

func (s *fakeSourceCodeStorage) PutTestCases(_ context.Context, _ repository.PutTestCasesParams) (string, error) {
	return "problems/1/testcases/content-hash.json", nil
}

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

func (r *fakeProblemRepository) Update(_ context.Context, params repository.UpdateProblemParams) (*domain.Problem, error) {
	return &domain.Problem{
		ID: params.ID, Title: params.Title, Statement: params.Statement, TimeLimitMS: params.TimeLimitMS,
		MemoryLimitKB: params.MemoryLimitKB, Visibility: params.Visibility, CreatedBy: routerUserID,
	}, nil
}

func (r *fakeProblemRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeProblemRepository) SetCurrentTestCasesKey(_ context.Context, _ int64, _ string) error {
	return nil
}

type fakeRouterTestCaseRepository struct{}

func (r *fakeRouterTestCaseRepository) CountByProblemID(_ context.Context, _ int64) (int, error) {
	return 2, nil
}

func (r *fakeRouterTestCaseRepository) ListByProblemID(_ context.Context, problemID int64) ([]domain.TestCase, error) {
	return []domain.TestCase{{ID: 7, ProblemID: problemID, Score: 1}}, nil
}

func (r *fakeRouterTestCaseRepository) FindByID(_ context.Context, problemID, id int64) (*domain.TestCase, error) {
	return &domain.TestCase{ID: id, ProblemID: problemID, Input: "1 2\n", ExpectedOutput: "3\n", IsSample: true, Score: 1}, nil
}

func (r *fakeRouterTestCaseRepository) Create(_ context.Context, params repository.CreateTestCaseParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: 7, ProblemID: params.ProblemID, Input: params.Input, ExpectedOutput: params.ExpectedOutput, IsSample: params.IsSample, Score: params.Score}, nil
}

func (r *fakeRouterTestCaseRepository) Update(_ context.Context, params repository.UpdateTestCaseParams) (*domain.TestCase, error) {
	return &domain.TestCase{ID: params.ID, ProblemID: params.ProblemID, Input: params.Input, ExpectedOutput: params.ExpectedOutput, IsSample: params.IsSample, Score: 1}, nil
}

func (r *fakeRouterTestCaseRepository) Delete(_ context.Context, _, _ int64) error {
	return nil
}
