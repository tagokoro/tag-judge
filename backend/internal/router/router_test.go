package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
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
	if body.Data.CreatedBy != "seed-author" {
		t.Fatalf("created_by = %q, want the creator's handle", body.Data.CreatedBy)
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
			UserID string  `json:"user_id"`
			Role   string  `json:"role"`
			Handle *string `json:"handle"`
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
	if body.Data.Handle == nil || *body.Data.Handle != "seed-author" {
		t.Fatalf("handle = %v", body.Data.Handle)
	}
}

func TestMeRouteWithoutHandle(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	options.profiles.profile = nil
	router := newTestRouterWithHandlers(options)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	setAuth(req, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"handle":null`) {
		t.Fatalf("GET /me = %d %s", res.Code, res.Body.String())
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
const adminUserID = "00000000-0000-4000-8000-000000000002"

func setAuth(req *http.Request, role string) {
	setAuthAsUser(req, routerUserID, role)
}

func setAuthAsUser(req *http.Request, userID string, role string) {
	req.Header.Set("X-User-ID", userID)
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
	return newTestRouterWithHandlers(newTestRouterOptions(groups, submissions))
}

type testRouterOptions struct {
	groups      repository.GroupRepository
	submissions repository.SubmissionRepository
	namespaces  repository.NamespaceRepository
	profiles    *routerUserProfileRepository
	libraries   *fakeLibraryRouterRepository
	versions    *fakeLibraryVersionRouterRepository
}

func newTestRouterOptions(groups repository.GroupRepository, submissions repository.SubmissionRepository) *testRouterOptions {
	libraries := newFakeLibraryRouterRepository(domain.LibraryVisibilityPublic)
	return &testRouterOptions{
		groups:      groups,
		submissions: submissions,
		namespaces:  newRouterNamespaceRepository(),
		profiles:    newRouterUserProfileRepository(),
		libraries:   libraries,
		versions:    newFakeLibraryVersionRouterRepository(),
	}
}

func newTestRouterWithHandlers(options *testRouterOptions) http.Handler {
	authRepo := &fakeAuthRepository{}
	authUsecase := usecase.NewAuthUsecase(authRepo, nil, 0)
	problemRepo := &fakeProblemRepository{}
	testCaseRepo := &fakeRouterTestCaseRepository{}
	admission := &fakeRouterAdmissionControl{}
	return New(Handlers{
		Problems:  handler.NewProblemHandler(usecase.NewProblemUsecase(problemRepo, testCaseRepo)),
		TestCases: handler.NewTestCaseHandler(usecase.NewTestCaseUsecase(problemRepo, testCaseRepo, &fakeSourceCodeStorage{})),
		CodeRuns: handler.NewCodeRunHandler(usecase.NewCodeRunUsecase(
			&fakeCodeRunRepository{},
			&fakeSourceCodeStorage{},
			admission,
			2500,
			255,
		)),
		Groups: handler.NewGroupHandler(usecase.NewGroupUsecase(options.groups)),
		Submissions: handler.NewSubmissionHandler(usecase.NewSubmissionUsecase(
			options.submissions,
			problemRepo,
			&fakeSourceCodeStorage{},
			admission,
			255,
		)),
		Me:         handler.NewMeHandler(usecase.NewUserProfileUsecase(options.profiles)),
		Profiles:   handler.NewUserProfileHandler(usecase.NewUserProfileUsecase(options.profiles)),
		AuthHTTP:   handler.NewAuthHandler(authUsecase, false, "/"),
		Auth:       appmiddleware.NewAuthMiddleware(authRepo, true),
		Namespaces: handler.NewNamespaceHandler(usecase.NewNamespaceUsecase(options.namespaces)),
		Libraries:  handler.NewLibraryHandler(usecase.NewLibraryUsecase(options.namespaces, options.libraries)),
		LibraryVersions: handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(
			usecase.NewLibraryUsecase(options.namespaces, options.libraries), options.versions,
		)),
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

func (r *fakeProblemRepository) ListByCreator(_ context.Context, params repository.ListProblemsByCreatorParams) ([]domain.Problem, error) {
	return []domain.Problem{{ID: 7, Title: "Mine", Visibility: domain.ProblemVisibilityPrivate, CreatedBy: params.CreatedBy, CreatedByHandle: "seed-author"}}, nil
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

func (r *fakeSubmissionRepository) ListRecentPublic(_ context.Context, _ repository.ListRecentSubmissionsParams) ([]domain.PublicSubmission, error) {
	return []domain.PublicSubmission{{ID: 9, ProblemID: 1, ProblemTitle: "A + B", Language: "python", Status: "ac", SubmitterHandle: "seed-author"}}, nil
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

type fakeRouterAdmissionControl struct{}

func (a *fakeRouterAdmissionControl) CreateSubmission(_ context.Context, params repository.CreateSubmissionWithOutboxParams, _ *repository.IdempotencyKeyInfo) (*domain.Submission, error) {
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

func (a *fakeRouterAdmissionControl) CreateCodeRun(_ context.Context, params repository.CreateCodeRunParams, _ *repository.IdempotencyKeyInfo, _ int, codeRunID int64) (*domain.CodeRun, error) {
	return &domain.CodeRun{
		ID:         codeRunID,
		UserID:     params.UserID,
		Language:   params.Language,
		SourceCode: params.SourceCode,
		Stdin:      params.Stdin,
		Status:     domain.CodeRunStatusPending,
	}, nil
}

func (a *fakeRouterAdmissionControl) ReserveCodeRunID(_ context.Context) (int64, error) {
	return 1, nil
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

func (r *fakeAuthRepository) UpsertGoogleUser(_ context.Context, _ repository.GoogleUser) (repository.GoogleLogin, error) {
	return repository.GoogleLogin{}, nil
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
		ID:              42,
		Title:           params.Title,
		Statement:       params.Statement,
		TimeLimitMS:     params.TimeLimitMS,
		MemoryLimitKB:   params.MemoryLimitKB,
		Visibility:      params.Visibility,
		CreatedBy:       params.CreatedBy,
		CreatedByHandle: "seed-author",
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

const submissionURLPath = "/api/v1/namespaces/seed-author/libraries/union-find/versions/1/submission-url"

func putSubmissionURL(t *testing.T, router http.Handler, userID, role, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, submissionURLPath, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		setAuthAsUser(req, userID, role)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func TestSubmissionURLRoutesOwnerSetsAndClears(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	router := newTestRouterWithHandlers(options)
	const url = "https://judge.example.com/submissions/7"

	if res := putSubmissionURL(t, router, "", "", `{"submission_url":"`+url+`"}`); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous PUT status = %d, want 401", res.Code)
	}
	res := putSubmissionURL(t, router, routerUserID, "user", `{"submission_url":"`+url+`"}`)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), url) {
		t.Fatalf("owner PUT = %d: %s", res.Code, res.Body.String())
	}
	for _, banned := range []string{"case_results", "verdict", `"score"`, `"status"`} {
		if strings.Contains(res.Body.String(), banned) {
			t.Fatalf("PUT response must not contain %q: %s", banned, res.Body.String())
		}
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/1", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), url) {
		t.Fatalf("anonymous version read must expose the URL: %d %s", get.Code, get.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, submissionURLPath, nil)
	setAuth(del, "user")
	delRes := httptest.NewRecorder()
	router.ServeHTTP(delRes, del)
	if delRes.Code != http.StatusNoContent {
		t.Fatalf("owner DELETE status = %d, want 204: %s", delRes.Code, delRes.Body.String())
	}
	if options.versions.byNumber[1].SubmissionURL != "" {
		t.Fatalf("DELETE left URL %q", options.versions.byNumber[1].SubmissionURL)
	}
}

func TestSubmissionURLRoutesRejectNonOwner(t *testing.T) {
	body := `{"submission_url":"https://judge.example.com/submissions/7"}`

	public := newTestRouterWithHandlers(newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{}))
	res := putSubmissionURL(t, public, adminUserID, string(domain.UserRoleAdmin), body)
	if res.Code != http.StatusForbidden {
		t.Fatalf("non-owner PUT on public library status = %d, want 403: %s", res.Code, res.Body.String())
	}
	assertErrorCode(t, res, "LIBRARY_FORBIDDEN")

	options := newTestRouterWithPrivateLibrary()
	private := newTestRouterWithHandlers(options)
	res = putSubmissionURL(t, private, adminUserID, string(domain.UserRoleAdmin), body)
	if res.Code != http.StatusNotFound {
		t.Fatalf("non-owner PUT on private library status = %d, want 404: %s", res.Code, res.Body.String())
	}
	if res := putSubmissionURL(t, private, routerUserID, "user", body); res.Code != http.StatusOK {
		t.Fatalf("owner PUT on private library status = %d, want 200: %s", res.Code, res.Body.String())
	}
}

func TestSubmissionURLRejectsInvalidURL(t *testing.T) {
	router := newTestRouter()
	for _, url := range []string{"ftp://example.com/x", "https://judge.example.com/" + strings.Repeat("a", domain.MaxSubmissionURLLength)} {
		body, err := json.Marshal(struct {
			SubmissionURL string `json:"submission_url"`
		}{SubmissionURL: url})
		if err != nil {
			t.Fatal(err)
		}
		res := putSubmissionURL(t, router, routerUserID, "user", string(body))
		if res.Code != http.StatusBadRequest {
			t.Fatalf("invalid URL status = %d, want 400: %s", res.Code, res.Body.String())
		}
		assertErrorCode(t, res, "INVALID_SUBMISSION_URL")
	}
}

func TestNamespaceRoutesRequiredAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/namespaces", body: `{"slug":"seed-author","display_name":"Seed Author"}`},
		{method: http.MethodGet, path: "/api/v1/me/namespace"},
	}
	for _, route := range routes {
		router := newTestRouter()
		req := httptest.NewRequest(route.method, route.path, bytes.NewBufferString(route.body))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", route.method, route.path, res.Code, http.StatusUnauthorized)
		}
	}
}

func TestCreateNamespaceRoute(t *testing.T) {
	repo := newRouterNamespaceRepository()
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	options.namespaces = repo
	router := newTestRouterWithHandlers(options)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", bytes.NewBufferString(`{"slug":"seed-author","display_name":"  Seed Author  "}`))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	if repo.createParams.DisplayName != "Seed Author" || repo.createParams.OwnerUserID != routerUserID {
		t.Fatalf("params = %+v", repo.createParams)
	}
	var body struct {
		Data struct {
			ID          string `json:"id"`
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			OwnerUserID string `json:"owner_user_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Slug != "seed-author" || body.Data.DisplayName != "Seed Author" || body.Data.OwnerUserID != routerUserID {
		t.Fatalf("data = %+v", body.Data)
	}
}

func TestCreateNamespaceRouteUnknownField(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", bytes.NewBufferString(`{"slug":"seed-author","display_name":"Seed Author","name":"extra"}`))
	setAuth(req, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, res, "INVALID_REQUEST_BODY")
}

func TestGetMyNamespaceRoute(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/namespace", nil)
	setAuth(req, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}
	var body struct {
		Data struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Slug != "seed-author" {
		t.Fatalf("slug = %q", body.Data.Slug)
	}
}

func TestGetMyNamespaceRouteNotFound(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	if repo, ok := options.namespaces.(*routerNamespaceRepository); ok {
		repo.byOwner = nil
	}
	router := newTestRouterWithHandlers(options)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/namespace", nil)
	setAuth(req, "author")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	assertErrorCode(t, res, "NAMESPACE_NOT_FOUND")
}

func TestGetNamespaceBySlugRoutePublic(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestGetNamespaceBySlugRouteNotFound(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	if repo, ok := options.namespaces.(*routerNamespaceRepository); ok {
		repo.bySlug = nil
	}
	router := newTestRouterWithHandlers(options)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/missing", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	assertErrorCode(t, res, "NAMESPACE_NOT_FOUND")
}

func TestLibraryRoutes(t *testing.T) {
	router := newTestRouter()

	create := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/seed-author/libraries", bytes.NewBufferString(`{"slug":"union-find","name":"UnionFind","description":"","visibility":"private"}`))
	req := create
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, "author")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, create)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		Data struct {
			ID            string `json:"id"`
			NamespaceSlug string `json:"namespace_slug"`
			Slug          string `json:"slug"`
			Visibility    string `json:"visibility"`
		} `json:"data"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Data.Slug != "union-find" || created.Data.NamespaceSlug != "seed-author" || created.Data.Visibility != "private" {
		t.Fatalf("data = %+v", created.Data)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find", bytes.NewBufferString(`{"visibility":"public"}`))
	patch.Header.Set("Content-Type", "application/json")
	setAuth(patch, "author")
	patchRes := httptest.NewRecorder()
	router.ServeHTTP(patchRes, patch)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("patch status = %d: %s", patchRes.Code, patchRes.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find", nil)
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, get)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d: %s", getRes.Code, getRes.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries?limit=3", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, list)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listRes.Code, listRes.Body.String())
	}
}

func TestLibraryRoutesRequireAuth(t *testing.T) {
	router := newTestRouter()
	for _, route := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/namespaces/seed-author/libraries", `{"slug":"union-find","name":"UnionFind","visibility":"public"}`},
		{http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find", `{"visibility":"public"}`},
	} {
		req := httptest.NewRequest(route.method, route.path, bytes.NewBufferString(route.body))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", route.method, route.path, res.Code, http.StatusUnauthorized)
		}
	}
}

func TestPrivateLibraryHiddenFromAdminViaHTTP(t *testing.T) {
	options := newTestRouterWithPrivateLibrary()
	router := newTestRouterWithHandlers(options)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find", nil)
	req.Header.Set("X-User-ID", adminUserID)
	req.Header.Set("X-User-Role", "admin")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, http.StatusNotFound, res.Body.String())
	}
	assertErrorCode(t, res, "LIBRARY_NOT_FOUND")

	list := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries", nil)
	list.Header.Set("X-User-ID", adminUserID)
	list.Header.Set("X-User-Role", "admin")
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, list)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRes.Code)
	}
	var body struct {
		Data []struct {
			Visibility string `json:"visibility"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listRes.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, library := range body.Data {
		if library.Visibility == "private" {
			t.Fatal("private library must not leak into other users' list")
		}
	}
}

func TestLibraryOwnerRequiredViaHTTP(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find", bytes.NewBufferString(`{"visibility":"public"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", adminUserID)
	req.Header.Set("X-User-Role", "admin")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
	assertErrorCode(t, res, "LIBRARY_FORBIDDEN")
}

func TestLibraryUpdateEmptyAndUnknownFields(t *testing.T) {
	router := newTestRouter()
	empty := httptest.NewRequest(http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find", bytes.NewBufferString(`{}`))
	empty.Header.Set("Content-Type", "application/json")
	setAuth(empty, "author")
	emptyRes := httptest.NewRecorder()
	router.ServeHTTP(emptyRes, empty)
	if emptyRes.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", emptyRes.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, emptyRes, "EMPTY_UPDATE")

	unknown := httptest.NewRequest(http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find", bytes.NewBufferString(`{"slug":"new-slug"}`))
	unknown.Header.Set("Content-Type", "application/json")
	setAuth(unknown, "author")
	unknownRes := httptest.NewRecorder()
	router.ServeHTTP(unknownRes, unknown)
	if unknownRes.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", unknownRes.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, unknownRes, "INVALID_REQUEST_BODY")
}

func TestLibraryListPaginationInvalid(t *testing.T) {
	router := newTestRouter()
	for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries"+query, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("query = %q, status = %d", query, res.Code)
		}
	}
}

func newTestRouterWithPrivateLibrary() *testRouterOptions {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	repo := newFakeLibraryRouterRepository(domain.LibraryVisibilityPrivate)
	options.libraries = repo
	return options
}

func TestLibraryVersionRoutes(t *testing.T) {
	router := newTestRouter()

	create := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/seed-author/libraries/union-find/versions", bytes.NewBufferString(`{"source_code":"def add(a, b):\n    return a + b\n"}`))
	create.Header.Set("Content-Type", "application/json")
	setAuth(create, "author")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, create)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createRes.Code, createRes.Body.String())
	}
	var created struct {
		Data struct {
			ID            string `json:"id"`
			VersionNumber int    `json:"version_number"`
			SourceCode    string `json:"source_code"`
			ContentSHA256 string `json:"content_sha256"`
			ByteSize      int    `json:"byte_size"`
		} `json:"data"`
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Data.VersionNumber != 1 || created.Data.SourceCode != "def add(a, b):\n    return a + b\n" || created.Data.ContentSHA256 == "" {
		t.Fatalf("data = %+v", created.Data)
	}

	detail := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/1", nil)
	detailRes := httptest.NewRecorder()
	router.ServeHTTP(detailRes, detail)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("detail status = %d: %s", detailRes.Code, detailRes.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions?limit=2&offset=1", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, list)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listRes.Code, listRes.Body.String())
	}
	var listBody struct {
		Data []struct {
			VersionNumber int `json:"version_number"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listRes.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listBody.Data) != 0 {
		t.Fatalf("list = %+v", listBody.Data)
	}
}

func TestLibraryVersionCreateRequiresAuth(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/seed-author/libraries/union-find/versions", bytes.NewBufferString(`{"source_code":"x"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestLibraryVersionDetailNotFound(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/9", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	assertErrorCode(t, res, "LIBRARY_VERSION_NOT_FOUND")
}

func TestLibraryVersionNumberValidation(t *testing.T) {
	router := newTestRouter()
	for _, raw := range []string{"abc", "0", "-1", "99999999999999999999999"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/"+raw, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("version %q status = %d, want 400", raw, res.Code)
		}
		assertErrorCode(t, res, "INVALID_VERSION_NUMBER")
	}
}

// TestLibraryVersionNumberRejectsInt32Overflow guards both paths that feed a
// version number to the repository. The sqlc query casts version_number to
// int32, so 4294967297 would wrap to version 1 without the handler bound.
func TestLibraryVersionNumberRejectsInt32Overflow(t *testing.T) {
	router := newTestRouter()
	for _, raw := range []string{"2147483648", "4294967296", "4294967297"} {
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/"+raw, nil)
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)
		if getRes.Code != http.StatusBadRequest {
			t.Fatalf("GET version %q status = %d, want 400", raw, getRes.Code)
		}
		assertErrorCode(t, getRes, "INVALID_VERSION_NUMBER")

		linkReq := httptest.NewRequest(http.MethodPut, "/api/v1/namespaces/seed-author/libraries/union-find/versions/"+raw+"/submission-url", bytes.NewBufferString(`{"submission_url":"https://judge.example.com/submissions/7"}`))
		linkReq.Header.Set("Content-Type", "application/json")
		setAuth(linkReq, "user")
		linkRes := httptest.NewRecorder()
		router.ServeHTTP(linkRes, linkReq)
		if linkRes.Code != http.StatusBadRequest {
			t.Fatalf("submission URL version %q status = %d, want 400: %s", raw, linkRes.Code, linkRes.Body.String())
		}
		assertErrorCode(t, linkRes, "INVALID_VERSION_NUMBER")
	}
}

// TestLibraryErrorMessagesMatchSpecContract pins the exact spec §10 messages
// returned to clients: the documented invalid cases (non-numeric/non-positive
// version, negative offset) keep their exact strings, while int32 overflow uses
// distinct stable 400 messages.
func TestLibraryErrorMessagesMatchSpecContract(t *testing.T) {
	router := newTestRouter()
	cases := []struct {
		name        string
		path        string
		wantCode    string
		wantMessage string
	}{
		{"non-positive version", "/api/v1/namespaces/seed-author/libraries/union-find/versions/0", "INVALID_VERSION_NUMBER", "version_number must be a positive integer"},
		{"non-numeric version", "/api/v1/namespaces/seed-author/libraries/union-find/versions/abc", "INVALID_VERSION_NUMBER", "version_number must be a positive integer"},
		{"version overflow", "/api/v1/namespaces/seed-author/libraries/union-find/versions/4294967297", "INVALID_VERSION_NUMBER", "version_number is too large"},
		{"negative offset", "/api/v1/namespaces/seed-author/libraries?offset=-1", "INVALID_OFFSET", "offset must be greater than or equal to 0"},
		{"offset overflow", "/api/v1/namespaces/seed-author/libraries?offset=4294967297", "INVALID_OFFSET", "offset is too large"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400: %s", tc.name, res.Code, res.Body.String())
		}
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("%s: decode response: %v", tc.name, err)
		}
		if body.Error.Code != tc.wantCode || body.Error.Message != tc.wantMessage {
			t.Fatalf("%s: got (%q, %q), want (%q, %q)", tc.name, body.Error.Code, body.Error.Message, tc.wantCode, tc.wantMessage)
		}
	}
}

// TestLibraryListOffsetRejectsInt32Overflow verifies every library list casts
// its offset to int32; an overflowing offset must produce a stable client error
// instead of silently wrapping to a different page.
func TestLibraryListOffsetRejectsInt32Overflow(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{
		"/api/v1/namespaces/seed-author/libraries",
		"/api/v1/namespaces/seed-author/libraries/union-find/versions",
	} {
		for _, raw := range []string{"2147483648", "4294967297"} {
			req := httptest.NewRequest(http.MethodGet, path+"?offset="+raw, nil)
			setAuth(req, "user")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("%s?offset=%s status = %d, want 400: %s", path, raw, res.Code, res.Body.String())
			}
			assertErrorCode(t, res, "INVALID_OFFSET")
		}
	}
}

func TestLibraryVersionMethodNotAllowed(t *testing.T) {
	router := newTestRouter()
	for _, pair := range []struct{ method, path string }{
		{http.MethodPatch, "/api/v1/namespaces/seed-author/libraries/union-find/versions/1"},
		{http.MethodDelete, "/api/v1/namespaces/seed-author/libraries/union-find/versions/1"},
	} {
		req := httptest.NewRequest(pair.method, pair.path, nil)
		setAuth(req, "author")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d, want 405", pair.method, res.Code)
		}
	}
}

type routerNamespaceRepository struct {
	createParams repository.CreateNamespaceParams
	byOwner      *domain.Namespace
	bySlug       *domain.Namespace
	findErr      error
}

func newRouterNamespaceRepository() *routerNamespaceRepository {
	return &routerNamespaceRepository{
		byOwner: &domain.Namespace{
			ID:          "11111111-1111-4111-8111-111111111111",
			OwnerUserID: routerUserID,
			Slug:        "seed-author",
			DisplayName: "Seed Author",
		},
		bySlug: &domain.Namespace{
			ID:          "11111111-1111-4111-8111-111111111111",
			OwnerUserID: routerUserID,
			Slug:        "seed-author",
			DisplayName: "Seed Author",
		},
	}
}

func (r *routerNamespaceRepository) Create(_ context.Context, params repository.CreateNamespaceParams) (*domain.Namespace, error) {
	r.createParams = params
	return &domain.Namespace{
		ID:          "11111111-1111-4111-8111-111111111111",
		OwnerUserID: params.OwnerUserID,
		Slug:        params.Slug,
		DisplayName: params.DisplayName,
	}, nil
}

func (r *routerNamespaceRepository) FindByOwner(_ context.Context, ownerUserID string) (*domain.Namespace, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.byOwner == nil {
		return nil, apperror.NotFound("NAMESPACE_NOT_FOUND", "namespace not found")
	}
	return r.byOwner, nil
}

func (r *routerNamespaceRepository) FindBySlug(_ context.Context, slug string) (*domain.Namespace, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.bySlug == nil {
		return nil, apperror.NotFound("NAMESPACE_NOT_FOUND", "namespace not found")
	}
	return r.bySlug, nil
}

type fakeLibraryRouterRepository struct {
	library      domain.Library
	updateParams repository.UpdateLibraryParams
	createParams repository.CreateLibraryParams
	createErr    error
	listPublic   []domain.Library
	listAll      []domain.Library
}

func newFakeLibraryRouterRepository(visibility domain.LibraryVisibility) *fakeLibraryRouterRepository {
	return &fakeLibraryRouterRepository{
		library: domain.Library{
			ID:            "22222222-2222-4222-8222-222222222222",
			NamespaceID:   "11111111-1111-4111-8111-111111111111",
			NamespaceSlug: "seed-author",
			Slug:          "union-find",
			Name:          "UnionFind",
			Visibility:    visibility,
		},
	}
}

func (r *fakeLibraryRouterRepository) Create(_ context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	library := r.library
	library.Slug = params.Slug
	library.Name = params.Name
	library.Description = params.Description
	library.Visibility = params.Visibility
	return &library, nil
}

func (r *fakeLibraryRouterRepository) Update(_ context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	r.updateParams = params
	library := r.library
	if params.Visibility != nil {
		library.Visibility = domain.LibraryVisibility(*params.Visibility)
	}
	if params.Name != nil {
		library.Name = *params.Name
	}
	return &library, nil
}

func (r *fakeLibraryRouterRepository) FindBySlug(_ context.Context, _, slug string) (*domain.Library, error) {
	library := r.library
	library.Slug = slug
	return &library, nil
}

func (r *fakeLibraryRouterRepository) ListPublic(_ context.Context, _ string, _, _ int) ([]domain.Library, error) {
	return r.listPublic, nil
}

func (r *fakeLibraryRouterRepository) ListAllPublic(_ context.Context, _, _ int) ([]domain.Library, error) {
	library := r.library
	library.NamespaceSlug = "seed-author"
	return []domain.Library{library}, nil
}

func (r *fakeLibraryRouterRepository) ListAll(_ context.Context, _ string, _, _ int) ([]domain.Library, error) {
	return r.listAll, nil
}

type fakeLibraryVersionRouterRepository struct {
	createParams repository.CreateLibraryVersionParams
	createErr    error
	createCalls  int
	byNumber     map[int]*domain.LibraryVersion
	summaries    []domain.LibraryVersion
	listParams   repository.ListLibraryVersionsParams
}

func newFakeLibraryVersionRouterRepository() *fakeLibraryVersionRouterRepository {
	return &fakeLibraryVersionRouterRepository{
		byNumber: map[int]*domain.LibraryVersion{
			1: {ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, SourceCode: "def add(a, b):\n    return a + b\n", ContentSHA256: "ba1a531f581d2e6094e978ed6f7aca7a8d92eeb62c6e7ad73ee692f7f18bc772", ByteSize: 31},
		},
	}
}

func (r *fakeLibraryVersionRouterRepository) Create(_ context.Context, params repository.CreateLibraryVersionParams) (*domain.LibraryVersion, error) {
	r.createCalls++
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.LibraryVersion{
		ID:            "33333333-3333-4333-8333-333333333333",
		LibraryID:     params.LibraryID,
		VersionNumber: 1,
		SourceCode:    params.SourceCode,
		ContentSHA256: domain.LibrarySourceSHA256(params.SourceCode),
		ByteSize:      len([]byte(params.SourceCode)),
		CreatedBy:     params.CreatedBy,
	}, nil
}

func (r *fakeLibraryVersionRouterRepository) FindByNumber(_ context.Context, _ string, versionNumber int, _ string) (*domain.LibraryVersion, error) {
	if version, ok := r.byNumber[versionNumber]; ok {
		return version, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
}

func (r *fakeLibraryVersionRouterRepository) ListSummaries(_ context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	r.listParams = params
	return r.summaries, nil
}

func (r *fakeLibraryVersionRouterRepository) SetSubmissionURL(_ context.Context, params repository.SetLibraryVersionSubmissionURLParams) (*domain.LibraryVersion, error) {
	version, ok := r.byNumber[params.VersionNumber]
	if !ok {
		return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
	}
	version.SubmissionURL = ""
	if params.SubmissionURL != nil {
		version.SubmissionURL = *params.SubmissionURL
	}
	return version, nil
}

func TestSiteWideListRoutes(t *testing.T) {
	router := newTestRouter()

	get := func(path string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authed {
			setAuth(req, "user")
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	if res := get("/api/v1/libraries", false); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"namespace_slug":"seed-author"`) {
		t.Fatalf("anonymous GET /libraries = %d %s", res.Code, res.Body.String())
	}

	// /submissions/recent must not be captured by /submissions/{submission_id}.
	res := get("/api/v1/submissions/recent", false)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"problem_title":"A + B"`) {
		t.Fatalf("anonymous GET /submissions/recent = %d %s", res.Code, res.Body.String())
	}
	for _, private := range []string{"user_id", "source_code"} {
		if strings.Contains(res.Body.String(), private) {
			t.Fatalf("recent submissions must not expose %q: %s", private, res.Body.String())
		}
	}

	if res := get("/api/v1/me/problems", false); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET /me/problems = %d, want 401", res.Code)
	}
	res = get("/api/v1/me/problems", true)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"created_by":"seed-author"`) || strings.Contains(res.Body.String(), routerUserID) {
		t.Fatalf("GET /me/problems = %d %s", res.Code, res.Body.String())
	}
}

func TestProfileRoutesRequireAuth(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/profile", bytes.NewBufferString(`{"handle":"seed-author"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("POST /me/profile status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestCreateMyProfileRoute(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	router := newTestRouterWithHandlers(options)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/profile", bytes.NewBufferString(`{"handle":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	setAuth(req, "user")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	if res.Body.String() != `{"data":{"handle":"alice"}}`+"\n" {
		t.Fatalf("body = %s", res.Body.String())
	}
	if options.profiles.createParams.UserID != routerUserID {
		t.Fatalf("params = %+v", options.profiles.createParams)
	}
}

func TestGetUserProfileRoute(t *testing.T) {
	router := newTestRouter()
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/users/seed-author", nil))
	if res.Code != http.StatusOK || res.Body.String() != `{"data":{"handle":"seed-author"}}`+"\n" {
		t.Fatalf("GET /users/seed-author = %d %s", res.Code, res.Body.String())
	}

	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/users/nobody", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("unknown handle status = %d, want 404", res.Code)
	}
	assertErrorCode(t, res, "USER_NOT_FOUND")
}

type routerUserProfileRepository struct {
	createParams repository.CreateUserProfileParams
	profile      *domain.UserProfile
}

// newRouterUserProfileRepository knows the router user as "seed-author".
func newRouterUserProfileRepository() *routerUserProfileRepository {
	return &routerUserProfileRepository{
		profile: &domain.UserProfile{UserID: routerUserID, Handle: "seed-author"},
	}
}

func (r *routerUserProfileRepository) Create(_ context.Context, params repository.CreateUserProfileParams) (*domain.UserProfile, error) {
	r.createParams = params
	return &domain.UserProfile{UserID: params.UserID, Handle: params.Handle}, nil
}

func (r *routerUserProfileRepository) FindByUserID(_ context.Context, userID string) (*domain.UserProfile, error) {
	if r.profile == nil || r.profile.UserID != userID {
		return nil, apperror.NotFound("USER_NOT_FOUND", "user not found")
	}
	return r.profile, nil
}

func (r *routerUserProfileRepository) FindByHandle(_ context.Context, handle string) (*domain.UserProfile, error) {
	if r.profile == nil || r.profile.Handle != handle {
		return nil, apperror.NotFound("USER_NOT_FOUND", "user not found")
	}
	return r.profile, nil
}
