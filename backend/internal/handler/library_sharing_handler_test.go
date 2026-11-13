package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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

	libraryFixtureSource = "def add(a, b):\n    return a + b\n"
)

const (
	handlerCategoryID = "aaaaaaaaaaaaaaaaaaaa"
	handlerLibraryID  = "bbbbbbbbbbbbbbbbbbbb"
)

type fakeCategoryRepo struct{}

func (r *fakeCategoryRepo) FindByPublicID(_ context.Context, publicID string) (*domain.Category, error) {
	return &domain.Category{
		ID: "11111111-1111-4111-8111-111111111111", PublicID: publicID, OwnerUserID: handlerAuthorID,
		OwnerHandle: "seed-author", Name: "グラフ", HasPublicLibrary: true,
	}, nil
}

func (r *fakeCategoryRepo) ListByOwner(_ context.Context, _ repository.ListCategoriesParams) ([]domain.Category, error) {
	return nil, nil
}

func (r *fakeCategoryRepo) Update(_ context.Context, params repository.UpdateCategoryParams) (*domain.Category, error) {
	return &domain.Category{ID: params.ID, PublicID: handlerCategoryID, OwnerUserID: handlerAuthorID}, nil
}

type fakeLibraryRepo struct {
	visibility domain.LibraryVisibility
}

func (r *fakeLibraryRepo) library() *domain.Library {
	return &domain.Library{
		ID: "22222222-2222-4222-8222-222222222222", PublicID: handlerLibraryID, CategoryPublicID: handlerCategoryID,
		CategoryName: "グラフ", OwnerUserID: handlerAuthorID, OwnerHandle: "seed-author", Name: "UnionFind", Visibility: r.visibility,
	}
}

func (r *fakeLibraryRepo) Create(_ context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	library := r.library()
	library.Name, library.Visibility = params.Name, params.Visibility
	return library, nil
}

func (r *fakeLibraryRepo) Update(_ context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	library := r.library()
	if params.Visibility != nil {
		library.Visibility = domain.LibraryVisibility(*params.Visibility)
	}
	return library, nil
}

func (r *fakeLibraryRepo) FindByPublicID(_ context.Context, publicID string) (*domain.Library, error) {
	if publicID != handlerLibraryID {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	return r.library(), nil
}

func (r *fakeLibraryRepo) ListByCategory(_ context.Context, _ repository.ListLibrariesParams) ([]domain.Library, error) {
	return []domain.Library{}, nil
}

func (r *fakeLibraryRepo) ListAllPublic(_ context.Context, _, _ int) ([]domain.Library, error) {
	return nil, nil
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
		ContentSHA256: domain.LibrarySourceSHA256(params.SourceCode),
		ByteSize:      len([]byte(params.SourceCode)),
		CreatedBy:     params.CreatedBy,
	}
	r.created = version
	return version, nil
}

func (r *fakeVersionRepo) FindByNumber(_ context.Context, _ string, versionNumber int, _ string) (*domain.LibraryVersion, error) {
	if version, ok := r.byNumber[versionNumber]; ok {
		return version, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
}

func (r *fakeVersionRepo) ListSummaries(_ context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	r.listParams = params
	return r.summaries, nil
}

func (r *fakeVersionRepo) SetSubmissionURL(_ context.Context, params repository.SetLibraryVersionSubmissionURLParams) (*domain.LibraryVersion, error) {
	version, ok := r.byNumber[params.VersionNumber]
	if !ok {
		return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
	}
	if params.SubmissionURL == nil {
		version.SubmissionURL, version.SubmissionURLUpdatedAt = "", nil
	} else {
		now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
		version.SubmissionURL, version.SubmissionURLUpdatedAt = *params.SubmissionURL, &now
	}
	return version, nil
}

func newVersionRouter() (http.Handler, *fakeVersionRepo) {
	categories := &fakeCategoryRepo{}
	libraries := &fakeLibraryRepo{visibility: domain.LibraryVisibilityPublic}
	versions := &fakeVersionRepo{
		byNumber: map[int]*domain.LibraryVersion{
			1: {ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, SourceCode: libraryFixtureSource},
		},
		summaries: []domain.LibraryVersion{
			{ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, ContentSHA256: "ba1a531f581d2e6094e978ed6f7aca7a8d92eeb62c6e7ad73ee692f7f18bc772", ByteSize: 31},
		},
	}
	auth := appmiddleware.NewAuthMiddleware(nil, true)
	router := chi.NewRouter()
	router.Use(auth.InjectActor)
	router.With(auth.RequireAuth).Post("/api/v1/libraries/{library_id}/versions", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)).Create)
	router.Get("/api/v1/libraries/{library_id}/versions", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)).List)
	router.Get("/api/v1/libraries/{library_id}/versions/{version_number}", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)).Get)
	router.With(auth.RequireAuth).Put("/api/v1/libraries/{library_id}/versions/{version_number}/submission-url", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)).SetSubmissionURL)
	router.With(auth.RequireAuth).Delete("/api/v1/libraries/{library_id}/versions/{version_number}/submission-url", handler.NewLibraryVersionHandler(usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)).ClearSubmissionURL)
	return router, versions
}

// TestSubmissionURLSetAndClearViaHTTP pins the response shape: the version
// carries submission_url / submission_url_updated_at (null when unset), PUT
// returns 200 with the version, and DELETE returns 204.
func TestSubmissionURLSetAndClearViaHTTP(t *testing.T) {
	router, _ := newVersionRouter()
	const path = "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions/1"

	readURL := func() map[string]any {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		var body struct {
			Data map[string]any `json:"data"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Data
	}
	if data := readURL(); data["submission_url"] != nil || data["submission_url_updated_at"] != nil {
		t.Fatalf("unset URL must serialize as null, got %+v", data)
	}

	put := httptest.NewRequest(http.MethodPut, path+"/submission-url", strings.NewReader(`{"submission_url":"https://atcoder.jp/submissions/1"}`))
	put.Header.Set("Content-Type", "application/json")
	put.Header.Set("X-User-ID", handlerAuthorID)
	put.Header.Set("X-User-Role", "author")
	putRes := httptest.NewRecorder()
	router.ServeHTTP(putRes, put)
	if putRes.Code != http.StatusOK {
		t.Fatalf("PUT status = %d: %s", putRes.Code, putRes.Body.String())
	}
	if data := readURL(); data["submission_url"] != "https://atcoder.jp/submissions/1" || data["submission_url_updated_at"] == nil {
		t.Fatalf("set URL = %+v", data)
	}

	del := httptest.NewRequest(http.MethodDelete, path+"/submission-url", nil)
	del.Header.Set("X-User-ID", handlerAuthorID)
	del.Header.Set("X-User-Role", "author")
	delRes := httptest.NewRecorder()
	router.ServeHTTP(delRes, del)
	if delRes.Code != http.StatusNoContent || delRes.Body.Len() != 0 {
		t.Fatalf("DELETE = %d %q, want 204 with empty body", delRes.Code, delRes.Body.String())
	}
	if data := readURL(); data["submission_url"] != nil {
		t.Fatalf("cleared URL = %+v", data)
	}

	anon := httptest.NewRequest(http.MethodPut, path+"/submission-url", strings.NewReader(`{"submission_url":"https://atcoder.jp/submissions/1"}`))
	anon.Header.Set("Content-Type", "application/json")
	anonRes := httptest.NewRecorder()
	router.ServeHTTP(anonRes, anon)
	if anonRes.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous PUT status = %d, want 401", anonRes.Code)
	}
}

func TestListLibraryVersionsOmitsSource(t *testing.T) {
	router, _ := newVersionRouter()

	list := httptest.NewRequest(http.MethodGet, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions", nil)
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

	detail := httptest.NewRequest(http.MethodGet, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions/1", nil)
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

	defaultList := httptest.NewRequest(http.MethodGet, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions", nil)
	defaultRes := httptest.NewRecorder()
	router.ServeHTTP(defaultRes, defaultList)
	if defaultRes.Code != http.StatusOK {
		t.Fatalf("status = %d", defaultRes.Code)
	}
	if versions.listParams.Limit != 20 {
		t.Fatalf("default limit = %d, want 20", versions.listParams.Limit)
	}

	for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions"+query, nil)
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions", bytes.NewBufferString(`{"source_code":"x","extra":1}`))
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

// TestCreateLibraryVersionAcceptsEscapedMaxSizeSource is the regression for the
// raw-byte body cap: a decoded source may be up to MaxSourceBytes bytes, but
// JSON escaping can expand it to ~6x that many raw bytes. The old 300001
// raw-byte cap rejected such a valid request; the bounded library decoder must
// accept it while the submission endpoint keeps its original contract.
func TestCreateLibraryVersionAcceptsEscapedMaxSizeSource(t *testing.T) {
	router, versions := newVersionRouter()
	source := strings.Repeat("<", domain.MaxLibrarySourceBytes) // 100000 decoded bytes
	body, err := json.Marshal(struct {
		SourceCode string `json:"source_code"`
	}{SourceCode: source})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) <= 300001 {
		t.Fatalf("escaped body is only %d bytes; the regression needs a body over 300001", len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions", bytes.NewReader(body))
	req.Header.Set("X-User-ID", handlerAuthorID)
	req.Header.Set("X-User-Role", "author")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", res.Code, res.Body.String())
	}
	if versions.created == nil || versions.created.SourceCode != source {
		t.Fatalf("source did not round-trip: %+v", versions.created)
	}
}

func TestCreateLibraryVersionRejectsOversizedBody(t *testing.T) {
	router, versions := newVersionRouter()
	valid, err := json.Marshal(struct {
		SourceCode string `json:"source_code"`
	}{SourceCode: "print(1)"})
	if err != nil {
		t.Fatal(err)
	}
	// Trailing whitespace is valid JSON but pushes the raw body past the bound.
	body := append(valid, []byte(strings.Repeat(" ", 700000))...)
	if int64(len(body)) <= 6*domain.MaxLibrarySourceBytes+64*1024 {
		t.Fatalf("body is too small to exercise the cap: %d", len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/libraries/bbbbbbbbbbbbbbbbbbbb/versions", bytes.NewReader(body))
	req.Header.Set("X-User-ID", handlerAuthorID)
	req.Header.Set("X-User-Role", "author")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Error.Code != "INVALID_REQUEST_BODY" {
		t.Fatalf("error code = %q, want INVALID_REQUEST_BODY", errBody.Error.Code)
	}
	if versions.created != nil {
		t.Fatal("oversized body must not reach the repository")
	}
}

// TestOpenAPINullableFieldsUseUnionTypes guards the OpenAPI 3.1 fix: the 3.0
// "nullable" keyword is invalid in 3.1 and would not describe the actual null
// responses emitted for other schemas.
func TestOpenAPINullableFieldsUseUnionTypes(t *testing.T) {
	content, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	if strings.Contains(string(content), "nullable:") {
		t.Fatal("OpenAPI 3.1 schemas must use type unions, not the 3.0 nullable keyword")
	}
}

// TestOpenAPISubmissionURLRequestAcceptsOnlyTheURL pins the PUT body: a
// required submission_url and nothing else.
func TestOpenAPISubmissionURLRequestAcceptsOnlyTheURL(t *testing.T) {
	content, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	text := string(content)
	start := strings.Index(text, "    SetSubmissionURLRequest:")
	end := strings.Index(text, "    ErrorResponse:")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("SetSubmissionURLRequest schema not found in openapi.yaml")
	}
	block := text[start:end]
	for _, want := range []string{"required: [submission_url]", "additionalProperties: false"} {
		if !strings.Contains(block, want) {
			t.Fatalf("SetSubmissionURLRequest must contain %q", want)
		}
	}
}
