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

func newTestRouter() http.Handler {
	return newTestRouterWithGroups(newRouterGroupRepository())
}

func newTestRouterWithGroups(groups repository.GroupRepository) http.Handler {
	return New(Handlers{
		Problems: handler.NewProblemHandler(usecase.NewProblemUsecase(&fakeProblemRepository{})),
		Groups:   handler.NewGroupHandler(usecase.NewGroupUsecase(groups)),
		Me:       handler.NewMeHandler(),
		Auth:     appmiddleware.NewAuthMiddleware(),
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
	return &domain.Problem{ID: id}, nil
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
