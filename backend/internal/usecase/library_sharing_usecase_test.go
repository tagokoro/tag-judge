package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
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
	listAllPublicParams *[2]int
	createParams        repository.CreateLibraryParams
	createErr           error
	createCalls         int
	updateParams        repository.UpdateLibraryParams
	updateErr           error
	updateCalls         int
	bySlug              *domain.Library
	findErr             error
	listPublic          []domain.Library
	listAll             []domain.Library
	listPublicName      string
	listAllName         string
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

func (r *fakeLibraryRepository) ListAllPublic(_ context.Context, limit, offset int) ([]domain.Library, error) {
	r.listAllPublicParams = &[2]int{limit, offset}
	return r.listPublic, nil
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
	// findActorID captures the actor user id threaded into FindByNumber so the
	// RED-GREEN wiring tests can assert the fetch is actor-scoped, not
	// unconditional.
	findActorID string
	setParams   []repository.SetLibraryVersionSubmissionURLParams
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
		CreatedBy:     params.CreatedBy,
	}, nil
}

func (r *fakeLibraryVersionRepository) FindByNumber(_ context.Context, _ string, versionNumber int, actorUserID string) (*domain.LibraryVersion, error) {
	r.findActorID = actorUserID
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

func (r *fakeLibraryVersionRepository) SetSubmissionURL(_ context.Context, params repository.SetLibraryVersionSubmissionURLParams) (*domain.LibraryVersion, error) {
	r.setParams = append(r.setParams, params)
	version := &domain.LibraryVersion{LibraryID: params.LibraryID, VersionNumber: params.VersionNumber}
	if params.SubmissionURL != nil {
		version.SubmissionURL = *params.SubmissionURL
	}
	return version, nil
}

func TestCreateLibraryVersionPassesSourceToRepository(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	source := "def add(a, b):\n    return a + b\n"
	version, err := use.Create(context.Background(), authorActor(), "seed-author", "union-find", source)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if versions.createParams.SourceCode != source {
		t.Fatalf("source code round-trip failed: %q", versions.createParams.SourceCode)
	}
	if versions.createParams.CreatedBy != authorUserID {
		t.Fatalf("created_by = %q", versions.createParams.CreatedBy)
	}
	if version.SourceCode != source {
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

// The following RED-GREEN wiring tests assert that every version-source read
// threads the reader's user id into the repository fetch. That is what lets the
// (now visibility-scoped) SQL statement authorize the CURRENT library state in
// the same statement as the source read, closing the Resolve/check -> fetch
// TOCTOU window. Before the fix, these fetches carried no actor id.
func TestGetLibraryVersionThreadsActorIntoFetch(t *testing.T) {
	for _, actor := range []auth.Actor{auth.Actor{}, authorActor(), adminActor()} {
		namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
		versions := &fakeLibraryVersionRepository{
			byNumber: map[int]*domain.LibraryVersion{1: {VersionNumber: 1, SourceCode: "x=1\n"}},
		}
		use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
		if _, err := use.Get(context.Background(), actor, "seed-author", "union-find", 1); err != nil {
			t.Fatalf("Get as %q: %v", actor.UserID, err)
		}
		if versions.findActorID != actor.UserID {
			t.Fatalf("Get actor %q: FindByNumber actorUserID = %q, want reader id", actor.UserID, versions.findActorID)
		}
	}
}

func TestListLibraryVersionSummariesThreadsActorIntoFetch(t *testing.T) {
	for _, actor := range []auth.Actor{auth.Actor{}, authorActor(), adminActor()} {
		namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
		versions := &fakeLibraryVersionRepository{summaries: []domain.LibraryVersion{{VersionNumber: 1}}}
		use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
		if _, err := use.List(context.Background(), actor, "seed-author", "union-find", 5, true, 0); err != nil {
			t.Fatalf("List as %q: %v", actor.UserID, err)
		}
		if versions.listParams.ActorUserID != actor.UserID {
			t.Fatalf("List actor %q: ListSummaries ActorUserID = %q, want reader id", actor.UserID, versions.listParams.ActorUserID)
		}
	}
}

func TestSetSubmissionURLNormalizesAndPassesOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	version, err := use.SetSubmissionURL(context.Background(), authorActor(), "seed-author", "union-find", 1, "  HTTPS://atcoder.jp/submissions/1  ")
	if err != nil {
		t.Fatalf("SetSubmissionURL: %v", err)
	}
	if version.SubmissionURL != "https://atcoder.jp/submissions/1" {
		t.Fatalf("submission_url = %q", version.SubmissionURL)
	}
	if len(versions.setParams) != 1 || versions.setParams[0].ActorUserID != authorUserID || versions.setParams[0].VersionNumber != 1 {
		t.Fatalf("set params = %+v", versions.setParams)
	}

	if err := use.ClearSubmissionURL(context.Background(), authorActor(), "seed-author", "union-find", 1); err != nil {
		t.Fatalf("ClearSubmissionURL: %v", err)
	}
	if len(versions.setParams) != 2 || versions.setParams[1].SubmissionURL != nil {
		t.Fatalf("clear must pass a nil URL, got %+v", versions.setParams)
	}
}

func TestSetSubmissionURLRequiresOwner(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	_, err := use.SetSubmissionURL(context.Background(), adminActor(), "seed-author", "union-find", 1, "https://example.com/x")
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	err = use.ClearSubmissionURL(context.Background(), adminActor(), "seed-author", "union-find", 1)
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, librariesPrivate), versions)
	_, err = use.SetSubmissionURL(context.Background(), adminActor(), "seed-author", "union-find", 1, "https://example.com/x")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if len(versions.setParams) != 0 {
		t.Fatalf("repository must not be called for non-owners, got %+v", versions.setParams)
	}
}

func TestSetSubmissionURLValidatesInput(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versions)
	_, err := use.SetSubmissionURL(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, "https://example.com/x")
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("anonymous set err = %v, want unauthorized", err)
	}
	_, err = use.SetSubmissionURL(context.Background(), authorActor(), "seed-author", "union-find", 0, "https://example.com/x")
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	for _, raw := range []string{"", "ftp://example.com/x", "https://?x", "https://exa mple.com"} {
		_, err = use.SetSubmissionURL(context.Background(), authorActor(), "seed-author", "union-find", 1, raw)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SUBMISSION_URL", "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters")
	}
	if len(versions.setParams) != 0 {
		t.Fatalf("repository must not be called for invalid input, got %+v", versions.setParams)
	}
}

func TestListAllPublicLibrariesNeedsNoActor(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)

	if _, err := use.ListAllPublic(context.Background(), 0, false, 0); err != nil {
		t.Fatalf("ListAllPublic: %v", err)
	}
	if libraries.listAllPublicParams == nil || *libraries.listAllPublicParams != [2]int{20, 0} {
		t.Fatalf("params = %v, want default limit 20 offset 0", libraries.listAllPublicParams)
	}
}

func TestListAllPublicLibrariesRejectsBadPaging(t *testing.T) {
	namespaces, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(namespaces, libraries)

	for _, c := range []struct {
		limit         int
		limitProvided bool
		offset        int
	}{{0, true, 0}, {101, true, 0}, {20, true, -1}} {
		if _, err := use.ListAllPublic(context.Background(), c.limit, c.limitProvided, c.offset); !errors.Is(err, apperror.ErrInvalidInput) {
			t.Fatalf("%+v err = %v, want invalid input", c, err)
		}
	}
	if libraries.listAllPublicParams != nil {
		t.Fatal("repository must not be called for rejected paging")
	}
}
