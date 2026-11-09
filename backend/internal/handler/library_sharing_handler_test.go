package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const (
	handlerAuthorID = "00000000-0000-4000-8000-000000000001"
	handlerAdminID  = "00000000-0000-4000-8000-000000000002"

	libraryFixtureSource = "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
	adapterFixtureSource = "a, b = map(int, input().split())\nprint(add(a, b))\n"
)

type fakeNamespaceRepo struct{}

func (r *fakeNamespaceRepo) Create(_ context.Context, params repository.CreateNamespaceParams) (*domain.Namespace, error) {
	return &domain.Namespace{OwnerUserID: params.OwnerUserID, Slug: params.Slug, DisplayName: params.DisplayName}, nil
}

func (r *fakeNamespaceRepo) FindByOwner(_ context.Context, ownerUserID string) (*domain.Namespace, error) {
	return &domain.Namespace{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: ownerUserID, Slug: "seed-author", DisplayName: "Seed Author"}, nil
}

func (r *fakeNamespaceRepo) FindBySlug(_ context.Context, _ string) (*domain.Namespace, error) {
	return &domain.Namespace{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: handlerAuthorID, Slug: "seed-author", DisplayName: "Seed Author"}, nil
}

type fakeLibraryRepo struct {
	visibility libraryVisibilityAlias
}

type libraryVisibilityAlias = domain.LibraryVisibility

func (r *fakeLibraryRepo) Create(_ context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	return &domain.Library{Slug: params.Slug, Name: params.Name, Visibility: params.Visibility}, nil
}

func (r *fakeLibraryRepo) Update(_ context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	visibility := ""
	if params.Visibility != nil {
		visibility = *params.Visibility
	}
	return &domain.Library{Slug: "union-find", Visibility: domain.LibraryVisibility(visibility)}, nil
}

func (r *fakeLibraryRepo) FindBySlug(_ context.Context, _, _ string) (*domain.Library, error) {
	return &domain.Library{
		ID:            "22222222-2222-4222-8222-222222222222",
		NamespaceID:   "11111111-1111-4111-8111-111111111111",
		NamespaceSlug: "seed-author",
		Slug:          "union-find",
		Name:          "UnionFind",
		Visibility:    r.visibility,
	}, nil
}

func (r *fakeLibraryRepo) ListPublic(_ context.Context, _ string, _, _ int) ([]domain.Library, error) {
	return []domain.Library{}, nil
}

func (r *fakeLibraryRepo) ListAll(_ context.Context, _ string, _, _ int) ([]domain.Library, error) {
	return []domain.Library{}, nil
}

type fakeVersionRepo struct {
	created    *domain.LibraryVersion
	byNumber   map[int]*domain.LibraryVersion
	summaries  []domain.LibraryVersion
	listParams repository.ListLibraryVersionsParams
}

func (r *fakeVersionRepo) Create(_ context.Context, params repository.CreateLibraryVersionParams) (*domain.LibraryVersion, error) {
	version := &domain.LibraryVersion{
		ID:            "33333333-3333-4333-8333-333333333333",
		LibraryID:     params.LibraryID,
		VersionNumber: 1,
		SourceCode:    params.SourceCode,
		ContentSHA256: params.ContentSHA256,
		ByteSize:      params.ByteSize,
		CreatedBy:     params.CreatedBy,
	}
	r.created = version
	return version, nil
}

func (r *fakeVersionRepo) FindByNumber(_ context.Context, _ string, versionNumber int) (*domain.LibraryVersion, error) {
	if version, ok := r.byNumber[versionNumber]; ok {
		return version, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
}

func (r *fakeVersionRepo) ListSummaries(_ context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	r.listParams = params
	return r.summaries, nil
}

func newVersionRouter() (http.Handler, *fakeVersionRepo) {
	namespaces := &fakeNamespaceRepo{}
	libraries := &fakeLibraryRepo{visibility: domain.LibraryVisibilityPublic}
	versions := &fakeVersionRepo{
		byNumber: map[int]*domain.LibraryVersion{
			1: {ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, SourceCode: libraryFixtureSource},
		},
		summaries: []domain.LibraryVersion{
			{ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, ContentSHA256: "b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af", ByteSize: 86},
		},
	}
	auth := appmiddleware.NewAuthMiddleware(nil, true)
	router := chi.NewRouter()
	router.Use(auth.InjectActor)
	router.With(auth.RequireAuth).Post("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)).Create)
	router.Get("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)).List)
	router.Get("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions/{version_number}", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)).Get)
	return router, versions
}

func TestListLibraryVersionsOmitsSource(t *testing.T) {
	router, _ := newVersionRouter()

	list := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, list)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listRes.Code, listRes.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(listRes.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("data = %+v", body.Data)
	}
	if _, exists := body.Data[0]["source_code"]; exists {
		t.Fatal("summary list must not contain source_code")
	}
	if body.Data[0]["version_number"].(float64) != 1 || body.Data[0]["content_sha256"] == "" {
		t.Fatalf("summary = %+v", body.Data[0])
	}

	detail := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions/1", nil)
	detailRes := httptest.NewRecorder()
	router.ServeHTTP(detailRes, detail)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("detail status = %d", detailRes.Code)
	}
	var detailBody struct {
		Data struct {
			SourceCode string `json:"source_code"`
		} `json:"data"`
	}
	if err := json.NewDecoder(detailRes.Body).Decode(&detailBody); err != nil {
		t.Fatal(err)
	}
	if detailBody.Data.SourceCode != libraryFixtureSource {
		t.Fatalf("detail source = %q", detailBody.Data.SourceCode)
	}
}

func TestListLibraryVersionsValidatesPagination(t *testing.T) {
	router, versions := newVersionRouter()

	defaultList := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions", nil)
	defaultRes := httptest.NewRecorder()
	router.ServeHTTP(defaultRes, defaultList)
	if defaultRes.Code != http.StatusOK {
		t.Fatalf("status = %d", defaultRes.Code)
	}
	if versions.listParams.Limit != 20 {
		t.Fatalf("default limit = %d, want 20", versions.listParams.Limit)
	}

	for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/seed-author/libraries/union-find/versions"+query, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d", query, res.Code)
		}
		var errBody struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
			t.Fatal(err)
		}
		want := "INVALID_LIMIT"
		if strings.Contains(query, "offset") {
			want = "INVALID_OFFSET"
		}
		if errBody.Error.Code != want {
			t.Fatalf("query %q code = %q, want %q", query, errBody.Error.Code, want)
		}
	}
}

func TestLibraryVersionRequestBodyShape(t *testing.T) {
	router, versions := newVersionRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/seed-author/libraries/union-find/versions", bytes.NewBufferString(`{"source_code":"x","extra":1}`))
	req.Header.Set("X-User-ID", handlerAuthorID)
	req.Header.Set("X-User-Role", "author")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if versions.created != nil {
		t.Fatal("invalid body must not reach the repository")
	}
}
