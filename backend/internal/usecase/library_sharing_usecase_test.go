package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const authorUserID = "00000000-0000-4000-8000-000000000001"
const adminUserID = "00000000-0000-4000-8000-000000000002"

func authorActor() auth.Actor {
	return auth.Actor{UserID: authorUserID, Role: domain.UserRoleAuthor}
}

func adminActor() auth.Actor {
	return auth.Actor{UserID: adminUserID, Role: domain.UserRoleAdmin}
}

type fakeNamespaceRepository struct {
	createParams repository.CreateNamespaceParams
	createErr    error

	byOwner    *domain.Namespace
	byOwnerErr error

	bySlug    *domain.Namespace
	bySlugErr error
}

func (r *fakeNamespaceRepository) Create(_ context.Context, params repository.CreateNamespaceParams) (*domain.Namespace, error) {
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.Namespace{
		ID:          "11111111-1111-4111-8111-111111111111",
		OwnerUserID: params.OwnerUserID,
		Slug:        params.Slug,
		DisplayName: params.DisplayName,
	}, nil
}

func (r *fakeNamespaceRepository) FindByOwner(_ context.Context, ownerUserID string) (*domain.Namespace, error) {
	if r.byOwnerErr != nil {
		return nil, r.byOwnerErr
	}
	if r.byOwner != nil {
		return r.byOwner, nil
	}
	return &domain.Namespace{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: ownerUserID, Slug: "seed-author", DisplayName: "Seed Author"}, nil
}

func (r *fakeNamespaceRepository) FindBySlug(_ context.Context, slug string) (*domain.Namespace, error) {
	if r.bySlugErr != nil {
		return nil, r.bySlugErr
	}
	if r.bySlug != nil {
		return r.bySlug, nil
	}
	return &domain.Namespace{ID: "11111111-1111-4111-8111-111111111111", OwnerUserID: authorUserID, Slug: slug, DisplayName: "Seed Author"}, nil
}

func assertAppError(t *testing.T, err error, wantSentinel error, wantCode, wantMessage string) {
	t.Helper()
	if !errors.Is(err, wantSentinel) {
		t.Fatalf("errors.Is(err, %v) = false, err = %v", wantSentinel, err)
	}
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err is not AppError: %v", err)
	}
	if appErr.Code != wantCode {
		t.Fatalf("code = %q, want %q", appErr.Code, wantCode)
	}
	if appErr.Message != wantMessage {
		t.Fatalf("message = %q, want %q", appErr.Message, wantMessage)
	}
}

type sentinelKind struct {
	name string
	err  error
}

func TestCreateNamespaceTrimsDisplayName(t *testing.T) {
	namespaces := &fakeNamespaceRepository{}
	usecase := usecase.NewNamespaceUsecase(namespaces)
	namespace, err := usecase.Create(context.Background(), authorActor(), "seed-author", "  Seed Author  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if namespaces.createParams.DisplayName != "Seed Author" {
		t.Fatalf("display name stored as %q, want trimmed value", namespaces.createParams.DisplayName)
	}
	if namespaces.createParams.OwnerUserID != authorUserID || namespaces.createParams.Slug != "seed-author" {
		t.Fatalf("params = %+v", namespaces.createParams)
	}
	if namespace == nil || namespace.Slug != "seed-author" {
		t.Fatalf("namespace = %+v", namespace)
	}
}

func TestCreateNamespaceRejectsSecondNamespace(t *testing.T) {
	namespaces := &fakeNamespaceRepository{
		createErr: apperror.Conflict("NAMESPACE_ALREADY_EXISTS", "user already has a namespace"),
	}
	use := usecase.NewNamespaceUsecase(namespaces)
	_, err := use.Create(context.Background(), authorActor(), "seed-author", "Seed Author")
	assertAppError(t, err, apperror.ErrConflict, "NAMESPACE_ALREADY_EXISTS", "user already has a namespace")

	namespaces = &fakeNamespaceRepository{
		createErr: apperror.Conflict("NAMESPACE_SLUG_TAKEN", "namespace slug is already taken"),
	}
	use = usecase.NewNamespaceUsecase(namespaces)
	_, err = use.Create(context.Background(), authorActor(), "seed-author", "Seed Author")
	assertAppError(t, err, apperror.ErrConflict, "NAMESPACE_SLUG_TAKEN", "namespace slug is already taken")
}

func TestCreateNamespaceRejectsSlug(t *testing.T) {
	namespaces := &fakeNamespaceRepository{}
	use := usecase.NewNamespaceUsecase(namespaces)
	for _, slug := range []string{"A", "-ab", "a--b", "a", strings.Repeat("a", 33), "", "seed_author", "seed--author"} {
		_, err := use.Create(context.Background(), authorActor(), slug, "Seed Author")
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SLUG", "slug must be 2-32 lowercase letters, digits, or single hyphens")
	}
	if namespaces.createParams.Slug != "" {
		t.Fatalf("repository Create called for invalid slugs: %+v", namespaces.createParams)
	}
}

func TestCreateNamespaceRejectsDisplayName(t *testing.T) {
	namespaces := &fakeNamespaceRepository{}
	use := usecase.NewNamespaceUsecase(namespaces)
	_, err := use.Create(context.Background(), authorActor(), "seed-author", "   ")
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_DISPLAY_NAME", "display_name must be 1-64 characters")
	_, err = use.Create(context.Background(), authorActor(), "seed-author", strings.Repeat("x", 65))
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_DISPLAY_NAME", "display_name must be 1-64 characters")
	if namespaces.createParams.DisplayName != "" {
		t.Fatal("repository must not be called")
	}
}

func TestCreateNamespaceRequiresAuth(t *testing.T) {
	use := usecase.NewNamespaceUsecase(&fakeNamespaceRepository{})
	_, err := use.Create(context.Background(), auth.Actor{}, "seed-author", "Seed Author")
	assertAppError(t, err, apperror.ErrUnauthorized, "UNAUTHORIZED", "authentication required")
}

func TestGetMyNamespaceNotFound(t *testing.T) {
	namespaces := &fakeNamespaceRepository{
		byOwnerErr: apperror.NotFound("NAMESPACE_NOT_FOUND", "namespace not found"),
	}
	use := usecase.NewNamespaceUsecase(namespaces)
	_, err := use.GetMine(context.Background(), authorActor())
	assertAppError(t, err, apperror.ErrNotFound, "NAMESPACE_NOT_FOUND", "namespace not found")
}

func TestGetMyNamespaceReturnsMine(t *testing.T) {
	namespaces := &fakeNamespaceRepository{}
	use := usecase.NewNamespaceUsecase(namespaces)
	namespace, err := use.GetMine(context.Background(), authorActor())
	if err != nil {
		t.Fatalf("GetMine: %v", err)
	}
	if namespace.OwnerUserID != authorUserID {
		t.Fatalf("namespace = %+v", namespace)
	}
}

func TestGetNamespaceBySlugPublic(t *testing.T) {
	use := usecase.NewNamespaceUsecase(&fakeNamespaceRepository{})
	namespace, err := use.GetBySlug(context.Background(), "seed-author")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if namespace.Slug != "seed-author" {
		t.Fatalf("namespace = %+v", namespace)
	}
}

type fakeLibraryRepository struct {
	createParams   repository.CreateLibraryParams
	createErr      error
	createCalls    int
	updateParams   repository.UpdateLibraryParams
	updateErr      error
	updateCalls    int
	bySlug         *domain.Library
	findErr        error
	listPublic     []domain.Library
	listAll        []domain.Library
	listPublicName string
	listAllName    string
}

func (r *fakeLibraryRepository) Create(_ context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	r.createCalls++
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.Library{
		ID:          "22222222-2222-4222-8222-222222222222",
		NamespaceID: "11111111-1111-4111-8111-111111111111",
		Slug:        params.Slug,
		Name:        params.Name,
		Description: params.Description,
		Visibility:  params.Visibility,
	}, nil
}

func (r *fakeLibraryRepository) Update(_ context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	r.updateCalls++
	r.updateParams = params
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	return &domain.Library{
		ID: "22222222-2222-4222-8222-222222222222", NamespaceSlug: "seed-author", Slug: "union-find",
	}, nil
}

func (r *fakeLibraryRepository) FindBySlug(_ context.Context, namespaceID, slug string) (*domain.Library, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.bySlug != nil {
		return r.bySlug, nil
	}
	return &domain.Library{
		ID: "22222222-2222-4222-8222-222222222222", NamespaceID: namespaceID, NamespaceSlug: "seed-author", Slug: slug,
	}, nil
}

func (r *fakeLibraryRepository) ListPublic(_ context.Context, namespaceID string, _, _ int) ([]domain.Library, error) {
	r.listPublicName = namespaceID
	return r.listPublic, nil
}

func (r *fakeLibraryRepository) ListAll(_ context.Context, namespaceID string, _, _ int) ([]domain.Library, error) {
	r.listAllName = namespaceID
	return r.listAll, nil
}

func libraryFixture(visibility domain.LibraryVisibility) (*fakeNamespaceRepository, *fakeLibraryRepository) {
	namespaces := &fakeNamespaceRepository{}
	libraries := &fakeLibraryRepository{
		bySlug: &domain.Library{
			ID:            "22222222-2222-4222-8222-222222222222",
			NamespaceID:   "11111111-1111-4111-8111-111111111111",
			NamespaceSlug: "seed-author",
			Slug:          "union-find",
			Name:          "UnionFind",
			Visibility:    visibility,
		},
	}
	return namespaces, libraries
}

func TestGetPrivateLibraryHidesFromAdmin(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Get(context.Background(), adminActor(), "seed-author", "union-find")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")

	_, err = use.Get(context.Background(), adminActor(), "seed-author", "union-find")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
}

func TestGetPrivateLibraryVisibleToOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	library, err := use.Get(context.Background(), authorActor(), "seed-author", "union-find")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if library.Visibility != domain.LibraryVisibilityPrivate {
		t.Fatalf("library = %+v", library)
	}
}

func TestGetPublicLibraryVisibleToAnyone(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	library, err := use.Get(context.Background(), auth.Actor{}, "seed-author", "union-find")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if library.Visibility != domain.LibraryVisibilityPublic {
		t.Fatalf("library = %+v", library)
	}
}

func TestListHidesPrivateFromOthers(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	result, err := use.List(context.Background(), adminActor(), "seed-author", 20, true, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = result
	if libraries.listPublicName == "" {
		t.Fatal("ListPublic must be called for non-owners")
	}
	if libraries.listAllName != "" {
		t.Fatal("ListAll must not be called for non-owners")
	}
}

func TestListShowsAllToOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	if _, err := use.List(context.Background(), authorActor(), "seed-author", 20, true, 0); err != nil {
		t.Fatalf("List: %v", err)
	}
	if libraries.listAllName == "" {
		t.Fatal("ListAll must be called for the owner")
	}
}

func TestUpdatePublicLibraryRequiresOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	visibility := "public"
	_, err := use.Update(context.Background(), adminActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{
		Visibility: &visibility,
	})
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	if libraries.updateCalls != 0 {
		t.Fatal("repository Update must not be called")
	}

	_, err = use.Update(context.Background(), authorActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{
		Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("owner Update: %v", err)
	}
	if libraries.updateParams.Visibility == nil || *libraries.updateParams.Visibility != "public" {
		t.Fatalf("params = %+v", libraries.updateParams)
	}
}

func TestUpdatePrivateLibraryHidesFromOthers(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	name := "Renamed"
	_, err := use.Update(context.Background(), adminActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{Name: &name})
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if libraries.updateCalls != 0 {
		t.Fatal("repository Update must not be called")
	}
}

func TestCreateLibraryValidatesContract(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "a--b", Name: "UnionFind", Visibility: "private",
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SLUG", "slug must be 2-48 lowercase letters, digits, or single hyphens")

	_, err = use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "union-find", Name: "   ", Visibility: "private",
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_NAME", "name must be 1-80 characters")

	_, err = use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "union-find", Name: "UnionFind", Description: strings.Repeat("d", 2001), Visibility: "private",
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_DESCRIPTION", "description must be at most 2000 characters")

	_, err = use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "union-find", Name: "UnionFind", Visibility: "group",
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VISIBILITY", "visibility must be public or private")

	if libraries.createCalls != 0 {
		t.Fatal("repository Create must not be called for invalid input")
	}
}

func TestCreateLibraryStoresTrimmedName(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "union-find", Name: " UnionFind ", Description: "", Visibility: "private",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if libraries.createParams.Name != "UnionFind" || libraries.createParams.Visibility != domain.LibraryVisibilityPrivate {
		t.Fatalf("params = %+v", libraries.createParams)
	}
}

func TestUpdateLibraryValidation(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Update(context.Background(), authorActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{})
	assertAppError(t, err, apperror.ErrInvalidInput, "EMPTY_UPDATE", "at least one field is required")

	_, err = use.Update(context.Background(), authorActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{
		Name: ptr(""),
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_NAME", "name must be 1-80 characters")

	_, err = use.Update(context.Background(), authorActor(), "seed-author", "union-find", usecase.UpdateLibraryInput{
		Visibility: ptr("unlisted"),
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VISIBILITY", "visibility must be public or private")

	if libraries.updateCalls != 0 {
		t.Fatal("repository Update must not be called for invalid input")
	}
}

func TestCreateLibrarySlugTakenError(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	libraries.createErr = apperror.Conflict("LIBRARY_SLUG_TAKEN", "library slug is already taken")
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Create(context.Background(), authorActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "union-find", Name: "UnionFind", Visibility: "private",
	})
	assertAppError(t, err, apperror.ErrConflict, "LIBRARY_SLUG_TAKEN", "library slug is already taken")
}

func TestCreateLibraryForbidsNonOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)
	_, err := use.Create(context.Background(), adminActor(), "seed-author", usecase.CreateLibraryInput{
		Slug: "their-lib", Name: "Their", Visibility: "public",
	})
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	if libraries.createCalls != 0 {
		t.Fatal("repository Create must not be called for non-owners")
	}
}

func TestAuthorizeOwner(t *testing.T) {
	namespaces, librariesPublic := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, librariesPublic)
	library, err := use.AuthorizeOwner(context.Background(), authorActor(), "seed-author", "union-find")
	if err != nil {
		t.Fatalf("AuthorizeOwner: %v", err)
	}
	if library.Slug != "union-find" {
		t.Fatalf("library = %+v", library)
	}

	_, err = use.AuthorizeOwner(context.Background(), adminActor(), "seed-author", "union-find")
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryUsecase(namespaces, librariesPrivate)
	_, err = use.AuthorizeOwner(context.Background(), adminActor(), "seed-author", "union-find")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
}

func ptr[T any](value T) *T {
	return &value
}

type fakeLibraryVersionRepository struct {
	createParams repository.CreateLibraryVersionParams
	createErr    error
	createCalls  int
	byNumber     map[int]*domain.LibraryVersion
	findErr      error
	summaries    []domain.LibraryVersion
	listParams   repository.ListLibraryVersionsParams
}

func (r *fakeLibraryVersionRepository) Create(_ context.Context, params repository.CreateLibraryVersionParams) (*domain.LibraryVersion, error) {
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
		ContentSHA256: params.ContentSHA256,
		ByteSize:      params.ByteSize,
		CreatedBy:     params.CreatedBy,
	}, nil
}

func (r *fakeLibraryVersionRepository) FindByNumber(_ context.Context, _ string, versionNumber int) (*domain.LibraryVersion, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if version, ok := r.byNumber[versionNumber]; ok {
		return version, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
}

func (r *fakeLibraryVersionRepository) ListSummaries(_ context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	r.listParams = params
	return r.summaries, nil
}

func TestCreateLibraryVersionStoresExactHash(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	source := "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
	version, err := use.Create(context.Background(), authorActor(), "seed-author", "union-find", source)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if versions.createParams.ContentSHA256 != "b24916acf9a8670a4e088af2bbc2591d02eca1f33ddad1aec0ed3a41d57568af" {
		t.Fatalf("content sha = %q", versions.createParams.ContentSHA256)
	}
	if versions.createParams.ByteSize != 86 {
		t.Fatalf("byte size = %d", versions.createParams.ByteSize)
	}
	if versions.createParams.SourceCode != source {
		t.Fatalf("source code round-trip failed: %q", versions.createParams.SourceCode)
	}
	if versions.createParams.CreatedBy != authorUserID {
		t.Fatalf("created_by = %q", versions.createParams.CreatedBy)
	}
	if version.SourceCode != source || version.ContentSHA256 != versions.createParams.ContentSHA256 {
		t.Fatalf("version = %+v", version)
	}
}

func TestCreateLibraryVersionRejectsMutationByNonOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	_, err := use.Create(context.Background(), adminActor(), "seed-author", "union-find", "def x():\n    pass\n")
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, librariesPrivate), versions)
	_, err = use.Create(context.Background(), adminActor(), "seed-author", "union-find", "def x():\n    pass\n")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}
}

func TestCreateLibraryVersionRejectsInvalidSource(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	const wantMessage = "source_code must be 1-100000 bytes, non-blank, and must not contain NUL"
	for _, source := range []string{"   ", "a\x00b", strings.Repeat("x", 100001)} {
		_, err := use.Create(context.Background(), authorActor(), "seed-author", "union-find", source)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SOURCE_CODE", wantMessage)
	}
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}
}

func TestGetLibraryVersion(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{
			1: {ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, SourceCode: "def add(a, b):\n    return a + b\n"},
		},
	}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	version, err := use.Get(context.Background(), auth.Actor{}, "seed-author", "union-find", 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if version.SourceCode != "def add(a, b):\n    return a + b\n" {
		t.Fatalf("version = %+v", version)
	}
}

func TestListLibraryVersionSummaries(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{
		summaries: []domain.LibraryVersion{{VersionNumber: 1}},
	}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	versionsList, err := use.List(context.Background(), auth.Actor{}, "seed-author", "union-find", 5, true, 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(versionsList) != 1 || versionsList[0].VersionNumber != 1 {
		t.Fatalf("versions = %+v", versionsList)
	}
	if versions.listParams.Limit != 5 || versions.listParams.Offset != 2 {
		t.Fatalf("list params = %+v", versions.listParams)
	}
}

type fakeProblemReaderForVerify struct {
	problem *domain.Problem
	err     error
}

func (r *fakeProblemReaderForVerify) FindPublicByID(_ context.Context, id int64) (*domain.Problem, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.problem, nil
}

type fakeVerificationStorage struct {
	put      repository.PutSourceCodeParams
	putErr   error
	putCalls int
}

func (s *fakeVerificationStorage) PutSourceCode(_ context.Context, params repository.PutSourceCodeParams) error {
	s.putCalls++
	s.put = params
	return s.putErr
}

type fakeSubmissionReaderForVerify struct {
	results          []domain.SubmissionCaseResult
	resultsErr       error
	caseResultsCalls int
	submission       *domain.Submission
	findErr          error
}

func (r *fakeSubmissionReaderForVerify) FindByID(_ context.Context, id int64) (*domain.Submission, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.submission != nil {
		return r.submission, nil
	}
	return &domain.Submission{ID: id, ProblemID: 42, Status: domain.SubmissionStatusPending}, nil
}

func (r *fakeSubmissionReaderForVerify) ListCaseResults(_ context.Context, _ int64) ([]domain.SubmissionCaseResult, error) {
	r.caseResultsCalls++
	return r.results, r.resultsErr
}

type fakeVerificationRepository struct {
	createParams repository.CreateLibraryVerificationParams
	createErr    error
	createCalls  int

	byID       map[int64]*domain.LibraryVerification
	findIDErr  error
	findCalls  int
	listUser   []domain.LibraryVerification
	listCalled bool
	listErr    error
}

func (r *fakeVerificationRepository) CreateWithSubmission(_ context.Context, params repository.CreateLibraryVerificationParams) (*domain.LibraryVerification, *domain.Submission, error) {
	r.createCalls++
	r.createParams = params
	if r.createErr != nil {
		return nil, nil, r.createErr
	}
	verification := &domain.LibraryVerification{
		ID:               15,
		LibraryVersionID: params.LibraryVersionID,
		NamespaceSlug:    "seed-author",
		LibrarySlug:      "union-find",
		ProblemID:        params.ProblemID,
		SubmissionID:     7,
		UserID:           params.ActorUserID,
		Language:         params.Language,
		Status:           domain.SubmissionStatusPending,
		Score:            0,
		AdapterSource:    params.AdapterSource,
		AdapterSHA256:    librarycombine.ContentSHA256(params.AdapterSource),
		VersionNumber:    1,
	}
	submission := &domain.Submission{ID: 7, UserID: params.ActorUserID, ProblemID: params.ProblemID, Language: params.Language, Status: domain.SubmissionStatusPending}
	return verification, submission, nil
}

func (r *fakeVerificationRepository) FindByID(_ context.Context, id int64) (*domain.LibraryVerification, error) {
	r.findCalls++
	if r.findIDErr != nil {
		return nil, r.findIDErr
	}
	if verification, ok := r.byID[id]; ok {
		return verification, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
}

func (r *fakeVerificationRepository) ListByUserID(_ context.Context, _ repository.ListLibraryVerificationsParams) ([]domain.LibraryVerification, error) {
	r.listCalled = true
	return r.listUser, r.listErr
}

var versionUnderTest = &domain.LibraryVersion{
	ID:            "33333333-3333-4333-8333-333333333333",
	LibraryID:     "22222222-2222-4222-8222-222222222222",
	VersionNumber: 1,
	SourceCode:    "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n",
}

func newVerificationUsecase(visibility domain.LibraryVisibility, problem domain.Problem, problemErr error) (*usecase.LibraryVerificationUsecase, *fakeLibraryVersionRepository, *fakeVerificationStorage, *fakeVerificationRepository, *fakeProblemReaderForVerify) {
	namespaces, libraries := libraryFixture(visibility)
	versionRepo := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{1: copyLibraryVersion(versionUnderTest)},
	}
	storage := &fakeVerificationStorage{}
	verificationRepo := &fakeVerificationRepository{}
	reader := &fakeProblemReaderForVerify{problem: &problem, err: problemErr}
	use := usecase.NewLibraryVerificationUsecase(
		usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versionRepo),
		reader,
		storage,
		verificationRepo,
		&fakeSubmissionReaderForVerify{},
	)
	return use, versionRepo, storage, verificationRepo, reader
}

func copyLibraryVersion(version *domain.LibraryVersion) *domain.LibraryVersion {
	copied := *version
	return &copied
}

var otherUserActor = auth.Actor{UserID: "00000000-0000-4000-8000-000000000007", Role: domain.UserRoleUser}

func publicProblem() domain.Problem {
	return domain.Problem{ID: 42, TimeLimitMS: 2000, CurrentTestCasesKey: "problems/42/testcases/abc.json"}
}

func TestVerifyPublicLibraryUsesExistingSubmissionPath(t *testing.T) {
	use, _, storage, repo, _ := newVerificationUsecase(domain.LibraryVisibilityPublic, publicProblem(), nil)

	adapter := "a, b = map(int, input().split())\nprint(add(a, b))\n"
	verification, err := use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 1, 42, adapter)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	expectedCombined, combineErr := librarycombine.Combine(versionUnderTest.SourceCode, adapter)
	if combineErr != nil {
		t.Fatalf("Combine: %v", combineErr)
	}
	if storage.putCalls != 1 {
		t.Fatalf("storage put calls = %d", storage.putCalls)
	}
	if storage.put.SourceCode != expectedCombined {
		t.Fatalf("storage source mismatch:\n%s", storage.put.SourceCode)
	}
	if len(storage.put.Key) == 0 {
		t.Fatal("storage key must not be empty")
	}

	params := repo.createParams
	if params.ActorUserID != otherUserActor.UserID {
		t.Fatalf("actor user id = %q", params.ActorUserID)
	}
	if params.TestCasesKey != "problems/42/testcases/abc.json" {
		t.Fatalf("test cases key = %q", params.TestCasesKey)
	}
	if params.TimeLimitMS != 2000 {
		t.Fatalf("time limit = %d", params.TimeLimitMS)
	}
	if params.Language != domain.SubmissionLanguagePython || params.ProblemID != 42 {
		t.Fatalf("params = %+v", params)
	}
	if params.AdapterSource != adapter {
		t.Fatalf("adapter passed through = %q", params.AdapterSource)
	}
	if params.LibraryVersionID != versionUnderTest.ID {
		t.Fatalf("version id = %q", params.LibraryVersionID)
	}

	if verification == nil || verification.Status != domain.SubmissionStatusPending || verification.Score != 0 {
		t.Fatalf("verification = %+v", verification)
	}
	if verification.ProblemID != 42 || verification.NamespaceSlug != "seed-author" || verification.LibrarySlug != "union-find" || verification.VersionNumber != 1 {
		t.Fatalf("verification = %+v", verification)
	}
	if verification.AdapterSHA256 != librarycombine.ContentSHA256(adapter) {
		t.Fatalf("adapter sha = %q", verification.AdapterSHA256)
	}
}

func TestVerifyPrivateLibraryIsHiddenFromAdmin(t *testing.T) {
	use, _, storage, repo, _ := newVerificationUsecase(domain.LibraryVisibilityPrivate, publicProblem(), nil)

	adapter := "print(add(1, 2))\n"
	_, err := use.Create(context.Background(), adminActor(), "seed-author", "union-find", 1, 42, adapter)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if storage.putCalls != 0 || repo.createCalls != 0 {
		t.Fatal("storage and repository must not be touched for a hidden library")
	}
}

func TestVerifyRejectsProblemWithoutCases(t *testing.T) {
	problem := publicProblem()
	problem.CurrentTestCasesKey = ""
	use, _, storage, repo, _ := newVerificationUsecase(domain.LibraryVisibilityPublic, problem, nil)

	_, err := use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 1, 42, "print(add(1, 2))\n")
	assertAppError(t, err, apperror.ErrConflict, "PROBLEM_HAS_NO_TEST_CASES", "problem has no test cases")
	if storage.putCalls != 0 || repo.createCalls != 0 {
		t.Fatal("nothing may be stored when the problem has no test cases")
	}

	// A private or missing problem never reaches storage either.
	use, _, storage, repo, _ = newVerificationUsecase(domain.LibraryVisibilityPublic, domain.Problem{}, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found"))
	_, err = use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 1, 42, "print(add(1, 2))\n")
	assertAppError(t, err, apperror.ErrNotFound, "PROBLEM_NOT_FOUND", "problem not found")
	if storage.putCalls != 0 || repo.createCalls != 0 {
		t.Fatal("storage and repository must not be touched for a missing problem")
	}
}

func TestVerifyRejectsInvalidAdapter(t *testing.T) {
	use, _, storage, repo, _ := newVerificationUsecase(domain.LibraryVisibilityPublic, publicProblem(), nil)
	const wantMessage = "adapter_source must be 1-100000 bytes, non-blank, and must not contain NUL"
	for _, adapter := range []string{"   ", "a\x00b", strings.Repeat("x", 100001)} {
		_, err := use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 1, 42, adapter)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_ADAPTER_SOURCE", wantMessage)
	}
	if storage.putCalls != 0 || repo.createCalls != 0 {
		t.Fatal("storage and repository must not be touched for an invalid adapter")
	}
}

func TestVerifyValidatesIdentifiers(t *testing.T) {
	use, _, storage, repo, reader := newVerificationUsecase(domain.LibraryVisibilityPublic, publicProblem(), nil)

	_, err := use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 1, 0, "print(add(1, 2))\n")
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_PROBLEM_ID", "problem_id must be a positive integer")

	_, err = use.Create(context.Background(), otherUserActor, "seed-author", "union-find", 0, 42, "print(add(1, 2))\n")
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VERSION_NUMBER", "version_number must be a positive integer")

	if repo.createCalls != 0 || storage.putCalls != 0 {
		t.Fatal("identifiers are validated before any side effect")
	}
	_ = reader
}

func TestGetVerificationHidesAdapterFromOtherUser(t *testing.T) {
	use, _ := newVerificationUsecaseForGet()
	adapterFix := "a, b = map(int, input().split())\nprint(add(a, b))\n"
	_, _, err := use.Get(context.Background(), otherUserActor, 15)
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_VERIFICATION_FORBIDDEN", "library verification is not accessible")

	verification, results, err := use.Get(context.Background(), authorActor(), 15)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if verification.AdapterSource != adapterFix {
		t.Fatalf("adapter source = %q", verification.AdapterSource)
	}
	if results == nil || len(results) != 0 {
		t.Fatalf("pending case results = %+v", results)
	}
}

func newVerificationUsecaseForGet() (*usecase.LibraryVerificationUsecase, *fakeSubmissionReaderForVerify) {
	nodes, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versionRepo := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{1: copyLibraryVersion(versionUnderTest)},
	}
	verificationRepo := &fakeVerificationRepository{
		byID: map[int64]*domain.LibraryVerification{
			15: {ID: 15, AdapterSource: "a, b = map(int, input().split())\nprint(add(a, b))\n", SubmissionID: 7, Status: domain.SubmissionStatusPending, ProblemID: 42, UserID: authorUserID},
		},
	}
	submissions := &fakeSubmissionReaderForVerify{}
	use := usecase.NewLibraryVerificationUsecase(
		usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(nodes, libraries), versionRepo),
		&fakeProblemReaderForVerify{},
		&fakeVerificationStorage{},
		verificationRepo,
		submissions,
	)
	return use, submissions
}

func TestGetVerificationNotFound(t *testing.T) {
	use, _ := newVerificationUsecaseForGet()
	_, _, err := use.Get(context.Background(), authorActor(), 999)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
}

func TestGetVerificationShowsCaseResults(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versionRepo := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{1: copyLibraryVersion(versionUnderTest)},
	}
	results := []domain.SubmissionCaseResult{{TestCaseID: 10, Status: domain.SubmissionStatusAC}}
	verificationRepo := &fakeVerificationRepository{
		byID: map[int64]*domain.LibraryVerification{
			15: {ID: 15, AdapterSource: "adapter", SubmissionID: 7, Status: domain.SubmissionStatusAC, ProblemID: 42, UserID: authorUserID},
		},
	}
	submissions := &fakeSubmissionReaderForVerify{results: results}
	use := usecase.NewLibraryVerificationUsecase(
		usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versionRepo),
		&fakeProblemReaderForVerify{},
		&fakeVerificationStorage{},
		verificationRepo,
		submissions,
	)

	_, got, err := use.Get(context.Background(), authorActor(), 15)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 1 || got[0].TestCaseID != 10 {
		t.Fatalf("case results = %+v", got)
	}
}

func TestListVerificationsUsesCallersOwnList(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versionRepo := &fakeLibraryVersionRepository{}
	verificationRepo := &fakeVerificationRepository{
		listUser: []domain.LibraryVerification{{ID: 15}},
	}
	submissions := &fakeSubmissionReaderForVerify{}
	use := usecase.NewLibraryVerificationUsecase(
		usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versionRepo),
		&fakeProblemReaderForVerify{},
		&fakeVerificationStorage{},
		verificationRepo,
		submissions,
	)

	verifications, err := use.List(context.Background(), otherUserActor, 10, true, 3)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !verificationRepo.listCalled {
		t.Fatal("ListByUserID must be called for the caller's own list")
	}
	if len(verifications) != 1 || verifications[0].ID != 15 {
		t.Fatalf("verifications = %+v", verifications)
	}
}
