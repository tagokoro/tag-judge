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

// Public ids as the API sees them, and the internal ids behind them.
const (
	nsID         = "aaaaaaaaaaaaaaaaaaaa"
	otherNsID    = "cccccccccccccccccccc"
	libID        = "bbbbbbbbbbbbbbbbbbbb"
	nsInternal   = "11111111-1111-4111-8111-111111111111"
	otherNsInt   = "44444444-4444-4444-8444-444444444444"
	libInternal  = "22222222-2222-4222-8222-222222222222"
	adminNsID    = "dddddddddddddddddddd"
	adminNsInter = "55555555-5555-4555-8555-555555555555"
)

func authorActor() auth.Actor {
	return auth.Actor{UserID: authorUserID, Role: domain.UserRoleAuthor}
}

func adminActor() auth.Actor {
	return auth.Actor{UserID: adminUserID, Role: domain.UserRoleAdmin}
}

type fakeCategoryRepository struct {
	// byID overrides what FindByPublicID returns for a public id.
	byID map[string]*domain.Category

	listParams   *repository.ListCategoriesParams
	updateParams *repository.UpdateCategoryParams
}

// FindByPublicID knows three categories unless byID overrides them: the
// author's nsID (with a public library) and otherNsID, and the admin's
// adminNsID.
func (r *fakeCategoryRepository) FindByPublicID(_ context.Context, publicID string) (*domain.Category, error) {
	if category, ok := r.byID[publicID]; ok {
		return category, nil
	}
	switch publicID {
	case nsID:
		return &domain.Category{ID: nsInternal, PublicID: nsID, OwnerUserID: authorUserID, OwnerHandle: "seed-author", Name: "グラフ", HasPublicLibrary: true}, nil
	case otherNsID:
		return &domain.Category{ID: otherNsInt, PublicID: otherNsID, OwnerUserID: authorUserID, OwnerHandle: "seed-author", Name: "数学"}, nil
	case adminNsID:
		return &domain.Category{ID: adminNsInter, PublicID: adminNsID, OwnerUserID: adminUserID, OwnerHandle: "seed-admin", Name: "Admin", HasPublicLibrary: true}, nil
	}
	return nil, apperror.NotFound("CATEGORY_NOT_FOUND", "category not found")
}

func (r *fakeCategoryRepository) ListByOwner(_ context.Context, params repository.ListCategoriesParams) ([]domain.Category, error) {
	r.listParams = &params
	return []domain.Category{{ID: nsInternal, PublicID: nsID, OwnerUserID: params.OwnerUserID, Name: "グラフ"}}, nil
}

func (r *fakeCategoryRepository) Update(_ context.Context, params repository.UpdateCategoryParams) (*domain.Category, error) {
	r.updateParams = &params
	category := &domain.Category{ID: params.ID, PublicID: nsID, OwnerUserID: authorUserID, Name: "グラフ"}
	if params.Name != nil {
		category.Name = *params.Name
	}
	if params.Description != nil {
		category.Description = *params.Description
	}
	return category, nil
}

// hiddenCategory is the author's nsID without a public library.
func hiddenCategory() map[string]*domain.Category {
	return map[string]*domain.Category{
		nsID: {ID: nsInternal, PublicID: nsID, OwnerUserID: authorUserID, OwnerHandle: "seed-author", Name: "グラフ"},
	}
}

type fakeUserProfileRepository struct {
	createParams *repository.CreateUserProfileParams
	createErr    error
	profile      *domain.UserProfile
}

func (r *fakeUserProfileRepository) Create(_ context.Context, params repository.CreateUserProfileParams) (*domain.UserProfile, error) {
	r.createParams = &params
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.UserProfile{UserID: params.UserID, Handle: params.Handle}, nil
}

func (r *fakeUserProfileRepository) FindByUserID(_ context.Context, userID string) (*domain.UserProfile, error) {
	if r.profile == nil || r.profile.UserID != userID {
		return nil, apperror.NotFound("USER_NOT_FOUND", "user not found")
	}
	return r.profile, nil
}

func (r *fakeUserProfileRepository) FindByHandle(_ context.Context, handle string) (*domain.UserProfile, error) {
	if r.profile == nil || r.profile.Handle != handle {
		return nil, apperror.NotFound("USER_NOT_FOUND", "user not found")
	}
	return r.profile, nil
}

func authorProfiles() *fakeUserProfileRepository {
	return &fakeUserProfileRepository{profile: &domain.UserProfile{UserID: authorUserID, Handle: "seed-author"}}
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

func TestCreateMyProfile(t *testing.T) {
	profiles := &fakeUserProfileRepository{}
	use := usecase.NewUserProfileUsecase(profiles)
	profile, err := use.CreateMine(context.Background(), authorActor(), "seed-author")
	if err != nil {
		t.Fatalf("CreateMine: %v", err)
	}
	if *profiles.createParams != (repository.CreateUserProfileParams{UserID: authorUserID, Handle: "seed-author"}) {
		t.Fatalf("params = %+v", profiles.createParams)
	}
	if profile.Handle != "seed-author" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestCreateMyProfileRejectsInput(t *testing.T) {
	profiles := &fakeUserProfileRepository{}
	use := usecase.NewUserProfileUsecase(profiles)
	for _, handle := range []string{"A", "-ab", "a--b", "a", strings.Repeat("a", 33), "", "seed_author", "Seed"} {
		_, err := use.CreateMine(context.Background(), authorActor(), handle)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_HANDLE", "handle must be 2-32 lowercase letters, digits, or single hyphens")
	}
	_, err := use.CreateMine(context.Background(), auth.Actor{}, "seed-author")
	assertAppError(t, err, apperror.ErrUnauthorized, "UNAUTHORIZED", "authentication required")
	if profiles.createParams != nil {
		t.Fatalf("repository Create called for invalid input: %+v", profiles.createParams)
	}
}

func TestFindMyProfileWithoutHandle(t *testing.T) {
	use := usecase.NewUserProfileUsecase(&fakeUserProfileRepository{})
	profile, err := use.FindMine(context.Background(), authorActor())
	if err != nil || profile != nil {
		t.Fatalf("FindMine = %+v, %v; want nil, nil", profile, err)
	}
}

func TestListMyCategoriesIncludesHidden(t *testing.T) {
	categories := &fakeCategoryRepository{}
	use := usecase.NewCategoryUsecase(categories, authorProfiles())
	if _, err := use.ListMine(context.Background(), authorActor()); err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	want := repository.ListCategoriesParams{OwnerUserID: authorUserID, IncludeHidden: true, Limit: domain.MaxCategoriesPerUser}
	if *categories.listParams != want {
		t.Fatalf("params = %+v, want %+v", categories.listParams, want)
	}
}

func TestListCategoriesByHandleHidesFromOthers(t *testing.T) {
	cases := []struct {
		name          string
		actor         auth.Actor
		includeHidden bool
	}{
		{name: "owner", actor: authorActor(), includeHidden: true},
		{name: "other user", actor: adminActor(), includeHidden: false},
		{name: "anonymous", actor: auth.Actor{}, includeHidden: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			categories := &fakeCategoryRepository{}
			use := usecase.NewCategoryUsecase(categories, authorProfiles())
			if _, err := use.ListByHandle(context.Background(), c.actor, "seed-author", 0, false, 0); err != nil {
				t.Fatalf("ListByHandle: %v", err)
			}
			want := repository.ListCategoriesParams{OwnerUserID: authorUserID, IncludeHidden: c.includeHidden, Limit: 20}
			if *categories.listParams != want {
				t.Fatalf("params = %+v, want %+v", categories.listParams, want)
			}
		})
	}
}

func TestListCategoriesByUnknownHandle(t *testing.T) {
	use := usecase.NewCategoryUsecase(&fakeCategoryRepository{}, authorProfiles())
	_, err := use.ListByHandle(context.Background(), authorActor(), "nobody", 0, false, 0)
	assertAppError(t, err, apperror.ErrNotFound, "USER_NOT_FOUND", "user not found")
}

func TestGetCategory(t *testing.T) {
	use := usecase.NewCategoryUsecase(&fakeCategoryRepository{}, authorProfiles())
	category, err := use.Get(context.Background(), auth.Actor{}, nsID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if category.OwnerHandle != "seed-author" || category.Name != "グラフ" {
		t.Fatalf("category = %+v", category)
	}
	for _, id := range []string{"missing", "eeeeeeeeeeeeeeeeeeee", "AAAAAAAAAAAAAAAAAAAA"} {
		_, err := use.Get(context.Background(), authorActor(), id)
		assertAppError(t, err, apperror.ErrNotFound, "CATEGORY_NOT_FOUND", "category not found")
	}
}

func TestGetHiddenCategoryOnlyForOwner(t *testing.T) {
	use := usecase.NewCategoryUsecase(&fakeCategoryRepository{byID: hiddenCategory()}, authorProfiles())
	if _, err := use.Get(context.Background(), authorActor(), nsID); err != nil {
		t.Fatalf("owner Get: %v", err)
	}
	for _, actor := range []auth.Actor{adminActor(), {}} {
		_, err := use.Get(context.Background(), actor, nsID)
		assertAppError(t, err, apperror.ErrNotFound, "CATEGORY_NOT_FOUND", "category not found")
	}
}

func TestUpdateCategoryByOwner(t *testing.T) {
	categories := &fakeCategoryRepository{}
	use := usecase.NewCategoryUsecase(categories, authorProfiles())
	name := "  グラフ理論  "
	category, err := use.Update(context.Background(), authorActor(), nsID, usecase.UpdateCategoryInput{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if categories.updateParams.ID != nsInternal || *categories.updateParams.Name != "グラフ理論" || categories.updateParams.Description != nil {
		t.Fatalf("params = %+v", categories.updateParams)
	}
	if category.OwnerHandle != "seed-author" || category.Name != "グラフ理論" {
		t.Fatalf("category = %+v", category)
	}
}

func TestUpdateCategoryRejects(t *testing.T) {
	use := usecase.NewCategoryUsecase(&fakeCategoryRepository{}, authorProfiles())
	_, err := use.Update(context.Background(), authorActor(), nsID, usecase.UpdateCategoryInput{})
	assertAppError(t, err, apperror.ErrInvalidInput, "EMPTY_UPDATE", "at least one field is required")

	name := "グラフ"
	_, err = use.Update(context.Background(), adminActor(), nsID, usecase.UpdateCategoryInput{Name: &name})
	assertAppError(t, err, apperror.ErrForbidden, "CATEGORY_FORBIDDEN", "category owner required")

	use = usecase.NewCategoryUsecase(&fakeCategoryRepository{byID: hiddenCategory()}, authorProfiles())
	_, err = use.Update(context.Background(), adminActor(), nsID, usecase.UpdateCategoryInput{Name: &name})
	assertAppError(t, err, apperror.ErrNotFound, "CATEGORY_NOT_FOUND", "category not found")
}

type fakeLibraryRepository struct {
	listAllPublicParams *[2]int
	createParams        repository.CreateLibraryParams
	createErr           error
	createCalls         int
	updateParams        repository.UpdateLibraryParams
	updateCalls         int
	library             *domain.Library
	listParams          *repository.ListLibrariesParams
}

func (r *fakeLibraryRepository) Create(_ context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	r.createCalls++
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	return &domain.Library{
		ID: libInternal, PublicID: libID, CategoryPublicID: nsID, CategoryName: params.CategoryName,
		OwnerUserID: params.OwnerUserID, OwnerHandle: "seed-author",
		Name: params.Name, Description: params.Description, Visibility: params.Visibility,
	}, nil
}

func (r *fakeLibraryRepository) Update(_ context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	r.updateCalls++
	r.updateParams = params
	library := *r.library
	return &library, nil
}

func (r *fakeLibraryRepository) FindByPublicID(_ context.Context, publicID string) (*domain.Library, error) {
	if r.library == nil || publicID != r.library.PublicID {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	library := *r.library
	return &library, nil
}

func (r *fakeLibraryRepository) ListByCategory(_ context.Context, params repository.ListLibrariesParams) ([]domain.Library, error) {
	r.listParams = &params
	return nil, nil
}

func (r *fakeLibraryRepository) ListAllPublic(_ context.Context, limit, offset int) ([]domain.Library, error) {
	r.listAllPublicParams = &[2]int{limit, offset}
	return nil, nil
}

// libraryFixture is the author's library libID in category nsID.
func libraryFixture(visibility domain.LibraryVisibility) (*fakeCategoryRepository, *fakeLibraryRepository) {
	categories := &fakeCategoryRepository{}
	libraries := &fakeLibraryRepository{
		library: &domain.Library{
			ID: libInternal, PublicID: libID, CategoryPublicID: nsID, CategoryName: "グラフ",
			OwnerUserID: authorUserID, OwnerHandle: "seed-author", Name: "UnionFind", Visibility: visibility,
		},
	}
	return categories, libraries
}

func TestGetPrivateLibraryHidesFromOthers(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(categories, libraries)
	for _, actor := range []auth.Actor{adminActor(), {}} {
		_, err := use.Get(context.Background(), actor, libID)
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
	library, err := use.Get(context.Background(), authorActor(), libID)
	if err != nil || library.Visibility != domain.LibraryVisibilityPrivate {
		t.Fatalf("owner Get = %+v, %v", library, err)
	}
}

func TestGetPublicLibraryVisibleToAnyone(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)
	library, err := use.Get(context.Background(), auth.Actor{}, libID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if library.OwnerHandle != "seed-author" || library.CategoryPublicID != nsID {
		t.Fatalf("library = %+v", library)
	}
	for _, id := range []string{"union-find", "BBBBBBBBBBBBBBBBBBBB", "eeeeeeeeeeeeeeeeeeee"} {
		_, err := use.Get(context.Background(), auth.Actor{}, id)
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
}

func TestListLibrariesScopesByActor(t *testing.T) {
	cases := []struct {
		name           string
		actor          auth.Actor
		includePrivate bool
	}{
		{name: "owner", actor: authorActor(), includePrivate: true},
		{name: "other user", actor: adminActor(), includePrivate: false},
		{name: "anonymous", actor: auth.Actor{}, includePrivate: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
			use := usecase.NewLibraryUsecase(categories, libraries)
			if _, err := use.List(context.Background(), c.actor, nsID, 20, true, 0); err != nil {
				t.Fatalf("List: %v", err)
			}
			want := repository.ListLibrariesParams{CategoryID: nsInternal, IncludePrivate: c.includePrivate, Limit: 20}
			if *libraries.listParams != want {
				t.Fatalf("params = %+v, want %+v", libraries.listParams, want)
			}
		})
	}
}

func TestHiddenCategoryLibrariesAreNotFoundForOthers(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	categories.byID = hiddenCategory()
	use := usecase.NewLibraryUsecase(categories, libraries)

	_, err := use.List(context.Background(), adminActor(), nsID, 20, true, 0)
	assertAppError(t, err, apperror.ErrNotFound, "CATEGORY_NOT_FOUND", "category not found")
	if _, err := use.List(context.Background(), authorActor(), nsID, 20, true, 0); err != nil {
		t.Fatalf("owner List: %v", err)
	}
}

func TestCreateLibrary(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(categories, libraries)
	library, err := use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{
		Name: " Union Find（素集合） ", Category: "  グラフ  ", Visibility: "private",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := repository.CreateLibraryParams{
		OwnerUserID: authorUserID, CategoryName: "グラフ", Name: "Union Find（素集合）", Visibility: domain.LibraryVisibilityPrivate,
	}
	if libraries.createParams != want {
		t.Fatalf("params = %+v, want %+v", libraries.createParams, want)
	}
	if library.PublicID != libID {
		t.Fatalf("library = %+v", library)
	}
}

func TestCreateLibraryPassesRepositoryConflicts(t *testing.T) {
	for _, conflict := range []error{
		apperror.Conflict("HANDLE_REQUIRED", "create a handle before creating a library"),
		apperror.Conflict("CATEGORY_LIMIT", "a user can own at most 20 categories"),
	} {
		categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
		libraries.createErr = conflict
		use := usecase.NewLibraryUsecase(categories, libraries)
		_, err := use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{Name: "X", Category: "グラフ", Visibility: "private"})
		if !errors.Is(err, conflict) {
			t.Fatalf("err = %v, want %v", err, conflict)
		}
	}
}

func TestCreateLibraryValidatesContract(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	use := usecase.NewLibraryUsecase(categories, libraries)
	_, err := use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{Name: "   ", Category: "グラフ", Visibility: "private"})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_NAME", "name must be 1-80 characters")
	for _, category := range []string{"", "   ", strings.Repeat("あ", 65)} {
		_, err = use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{Name: "UnionFind", Category: category, Visibility: "private"})
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_CATEGORY", "category must be 1-64 characters")
	}
	_, err = use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{
		Name: "UnionFind", Category: "グラフ", Description: strings.Repeat("d", 2001), Visibility: "private",
	})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_DESCRIPTION", "description must be at most 2000 characters")
	_, err = use.Create(context.Background(), authorActor(), usecase.CreateLibraryInput{Name: "UnionFind", Category: "グラフ", Visibility: "group"})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VISIBILITY", "visibility must be public or private")
	_, err = use.Create(context.Background(), auth.Actor{}, usecase.CreateLibraryInput{Name: "UnionFind", Category: "グラフ", Visibility: "private"})
	assertAppError(t, err, apperror.ErrUnauthorized, "UNAUTHORIZED", "authentication required")
	if libraries.createCalls != 0 {
		t.Fatal("repository Create must not be called for invalid input")
	}
}

func TestUpdateLibraryRequiresOwner(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)
	visibility := "private"
	_, err := use.Update(context.Background(), adminActor(), libID, usecase.UpdateLibraryInput{Visibility: &visibility})
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryUsecase(categories, librariesPrivate)
	_, err = use.Update(context.Background(), adminActor(), libID, usecase.UpdateLibraryInput{Visibility: &visibility})
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if libraries.updateCalls != 0 || librariesPrivate.updateCalls != 0 {
		t.Fatal("repository Update must not be called for non-owners")
	}

	use = usecase.NewLibraryUsecase(categories, libraries)
	if _, err := use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{Visibility: &visibility}); err != nil {
		t.Fatalf("owner Update: %v", err)
	}
	if libraries.updateParams.ID != libInternal || *libraries.updateParams.Visibility != "private" || libraries.updateParams.CategoryName != nil {
		t.Fatalf("params = %+v", libraries.updateParams)
	}
}

func TestMoveLibrary(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)
	target := " 数学 "
	if _, err := use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{Category: &target}); err != nil {
		t.Fatalf("move: %v", err)
	}
	if libraries.updateParams.CategoryName == nil || *libraries.updateParams.CategoryName != "数学" || libraries.updateParams.OwnerUserID != authorUserID {
		t.Fatalf("params = %+v, want the trimmed category name and the owner", libraries.updateParams)
	}

	libraries.updateCalls = 0
	empty := ""
	_, err := use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{Category: &empty})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_CATEGORY", "category must be 1-64 characters")
	_, err = use.Update(context.Background(), adminActor(), libID, usecase.UpdateLibraryInput{Category: &target})
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	if libraries.updateCalls != 0 {
		t.Fatal("rejected moves must not reach the repository")
	}
}

func TestUpdateLibraryValidation(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)
	_, err := use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{})
	assertAppError(t, err, apperror.ErrInvalidInput, "EMPTY_UPDATE", "at least one field is required")
	_, err = use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{Name: ptr("")})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_NAME", "name must be 1-80 characters")
	_, err = use.Update(context.Background(), authorActor(), libID, usecase.UpdateLibraryInput{Visibility: ptr("unlisted")})
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VISIBILITY", "visibility must be public or private")
	if libraries.updateCalls != 0 {
		t.Fatal("repository Update must not be called for invalid input")
	}
}

func TestAuthorizeOwner(t *testing.T) {
	categories, librariesPublic := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, librariesPublic)
	library, err := use.AuthorizeOwner(context.Background(), authorActor(), libID)
	if err != nil || library.ID != libInternal {
		t.Fatalf("AuthorizeOwner = %+v, %v", library, err)
	}
	_, err = use.AuthorizeOwner(context.Background(), adminActor(), libID)
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryUsecase(categories, librariesPrivate)
	_, err = use.AuthorizeOwner(context.Background(), adminActor(), libID)
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
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	source := "def add(a, b):\n    return a + b\n"
	version, err := use.Create(context.Background(), authorActor(), libID, source)
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
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	_, err := use.Create(context.Background(), adminActor(), libID, "def x():\n    pass\n")
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, librariesPrivate), versions)
	_, err = use.Create(context.Background(), adminActor(), libID, "def x():\n    pass\n")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}
}

func TestCreateLibraryVersionRejectsInvalidSource(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPrivate)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	const wantMessage = "source_code must be 1-100000 bytes, non-blank, and must not contain NUL"
	for _, source := range []string{"   ", "a\x00b", strings.Repeat("x", 100001)} {
		_, err := use.Create(context.Background(), authorActor(), libID, source)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SOURCE_CODE", wantMessage)
	}
	if versions.createCalls != 0 {
		t.Fatal("repository Create must not be called")
	}
}

func TestGetLibraryVersion(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{
			1: {ID: "33333333-3333-4333-8333-333333333333", VersionNumber: 1, SourceCode: "def add(a, b):\n    return a + b\n"},
		},
	}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	version, err := use.Get(context.Background(), auth.Actor{}, libID, 1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if version.SourceCode != "def add(a, b):\n    return a + b\n" {
		t.Fatalf("version = %+v", version)
	}
}

func TestListLibraryVersionSummaries(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{
		summaries: []domain.LibraryVersion{{VersionNumber: 1}},
	}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	versionsList, err := use.List(context.Background(), auth.Actor{}, libID, 5, true, 2)
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
		categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
		versions := &fakeLibraryVersionRepository{
			byNumber: map[int]*domain.LibraryVersion{1: {VersionNumber: 1, SourceCode: "x=1\n"}},
		}
		use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
		if _, err := use.Get(context.Background(), actor, libID, 1); err != nil {
			t.Fatalf("Get as %q: %v", actor.UserID, err)
		}
		if versions.findActorID != actor.UserID {
			t.Fatalf("Get actor %q: FindByNumber actorUserID = %q, want reader id", actor.UserID, versions.findActorID)
		}
	}
}

func TestListLibraryVersionSummariesThreadsActorIntoFetch(t *testing.T) {
	for _, actor := range []auth.Actor{auth.Actor{}, authorActor(), adminActor()} {
		categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
		versions := &fakeLibraryVersionRepository{summaries: []domain.LibraryVersion{{VersionNumber: 1}}}
		use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
		if _, err := use.List(context.Background(), actor, libID, 5, true, 0); err != nil {
			t.Fatalf("List as %q: %v", actor.UserID, err)
		}
		if versions.listParams.ActorUserID != actor.UserID {
			t.Fatalf("List actor %q: ListSummaries ActorUserID = %q, want reader id", actor.UserID, versions.listParams.ActorUserID)
		}
	}
}

func TestSetSubmissionURLNormalizesAndPassesOwner(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	version, err := use.SetSubmissionURL(context.Background(), authorActor(), libID, 1, "  HTTPS://atcoder.jp/submissions/1  ")
	if err != nil {
		t.Fatalf("SetSubmissionURL: %v", err)
	}
	if version.SubmissionURL != "https://atcoder.jp/submissions/1" {
		t.Fatalf("submission_url = %q", version.SubmissionURL)
	}
	if len(versions.setParams) != 1 || versions.setParams[0].ActorUserID != authorUserID || versions.setParams[0].VersionNumber != 1 {
		t.Fatalf("set params = %+v", versions.setParams)
	}

	if err := use.ClearSubmissionURL(context.Background(), authorActor(), libID, 1); err != nil {
		t.Fatalf("ClearSubmissionURL: %v", err)
	}
	if len(versions.setParams) != 2 || versions.setParams[1].SubmissionURL != nil {
		t.Fatalf("clear must pass a nil URL, got %+v", versions.setParams)
	}
}

func TestSetSubmissionURLRequiresOwner(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	_, err := use.SetSubmissionURL(context.Background(), adminActor(), libID, 1, "https://example.com/x")
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")
	err = use.ClearSubmissionURL(context.Background(), adminActor(), libID, 1)
	assertAppError(t, err, apperror.ErrForbidden, "LIBRARY_FORBIDDEN", "library owner required")

	_, librariesPrivate := libraryFixture(domain.LibraryVisibilityPrivate)
	use = usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, librariesPrivate), versions)
	_, err = use.SetSubmissionURL(context.Background(), adminActor(), libID, 1, "https://example.com/x")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if len(versions.setParams) != 0 {
		t.Fatalf("repository must not be called for non-owners, got %+v", versions.setParams)
	}
}

func TestSetSubmissionURLValidatesInput(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	versions := &fakeLibraryVersionRepository{}
	use := usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(categories, libraries), versions)
	_, err := use.SetSubmissionURL(context.Background(), auth.Actor{}, libID, 1, "https://example.com/x")
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("anonymous set err = %v, want unauthorized", err)
	}
	_, err = use.SetSubmissionURL(context.Background(), authorActor(), libID, 0, "https://example.com/x")
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	for _, raw := range []string{"", "ftp://example.com/x", "https://?x", "https://exa mple.com"} {
		_, err = use.SetSubmissionURL(context.Background(), authorActor(), libID, 1, raw)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SUBMISSION_URL", "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters")
	}
	if len(versions.setParams) != 0 {
		t.Fatalf("repository must not be called for invalid input, got %+v", versions.setParams)
	}
}

func TestListAllPublicLibrariesNeedsNoActor(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)

	if _, err := use.ListAllPublic(context.Background(), 0, false, 0); err != nil {
		t.Fatalf("ListAllPublic: %v", err)
	}
	if libraries.listAllPublicParams == nil || *libraries.listAllPublicParams != [2]int{20, 0} {
		t.Fatalf("params = %v, want default limit 20 offset 0", libraries.listAllPublicParams)
	}
}

func TestListAllPublicLibrariesRejectsBadPaging(t *testing.T) {
	categories, libraries := libraryFixture(domain.LibraryVisibilityPublic)
	use := usecase.NewLibraryUsecase(categories, libraries)

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
