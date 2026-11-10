package usecase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const (
	validSubmissionURL = "https://judge.example.com/submissions/42"
	// verificationURLInvalidMessage is the stable 400 contract shared with the
	// PostgreSQL adapter's URL validation backstop.
	verificationURLInvalidMessage = "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters"
)

var verificationOtherActor = auth.Actor{UserID: "00000000-0000-4000-8000-000000000007", Role: domain.UserRoleUser}

// verificationVersion is the one library version every link test resolves.
var verificationVersion = &domain.LibraryVersion{
	ID:            "33333333-3333-4333-8333-333333333333",
	LibraryID:     "22222222-2222-4222-8222-222222222222",
	VersionNumber: 1,
	SourceCode:    "def add(a, b):\n    return a + b\n",
}

// fakeVerificationRepository implements the URL-link repository contract. It
// has no Problem, Submission, Outbox, storage, or Judge surface on purpose: a
// link registration must be the only write the usecase can perform.
type fakeVerificationRepository struct {
	createParams repository.CreateLibraryVerificationParams
	createErr    error
	createCalls  int
	created      *domain.LibraryVerification

	byID      map[int64]*domain.LibraryVerification
	findErr   error
	findCalls int

	listResult []domain.LibraryVerification
	listParams repository.ListLibraryVerificationsParams
	listErr    error
	listCalled bool
}

func (r *fakeVerificationRepository) Create(_ context.Context, params repository.CreateLibraryVerificationParams) (*domain.LibraryVerification, error) {
	r.createCalls++
	r.createParams = params
	if r.createErr != nil {
		return nil, r.createErr
	}
	r.created = &domain.LibraryVerification{
		ID:               15,
		LibraryVersionID: params.LibraryVersionID,
		SubmissionURL:    params.SubmissionURL,
		CreatedBy:        params.ActorUserID,
		NamespaceSlug:    "seed-author",
		LibrarySlug:      "union-find",
		VersionNumber:    1,
		Visibility:       domain.LibraryVisibilityPublic,
		OwnerUserID:      authorUserID,
	}
	return r.created, nil
}

func (r *fakeVerificationRepository) FindByID(_ context.Context, id int64) (*domain.LibraryVerification, error) {
	r.findCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	if link, ok := r.byID[id]; ok {
		return link, nil
	}
	return nil, apperror.NotFound("LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
}

func (r *fakeVerificationRepository) ListByVersion(_ context.Context, params repository.ListLibraryVerificationsParams) ([]domain.LibraryVerification, error) {
	r.listCalled = true
	r.listParams = params
	if r.listErr != nil {
		return nil, r.listErr
	}
	if r.listResult != nil {
		return r.listResult, nil
	}
	return []domain.LibraryVerification{}, nil
}

func newVerificationLinkUsecase(visibility domain.LibraryVisibility) (*usecase.LibraryVerificationUsecase, *fakeLibraryVersionRepository, *fakeVerificationRepository) {
	namespaces, libraries := libraryFixture(visibility)
	versionRepo := &fakeLibraryVersionRepository{
		byNumber: map[int]*domain.LibraryVersion{1: verificationVersion},
	}
	verificationRepo := &fakeVerificationRepository{}
	use := usecase.NewLibraryVerificationUsecase(
		usecase.NewLibraryVersionUsecase(usecase.NewLibraryUsecase(namespaces, libraries), versionRepo),
		verificationRepo,
	)
	return use, versionRepo, verificationRepo
}

func publicLink() domain.LibraryVerification {
	return domain.LibraryVerification{
		ID:               15,
		LibraryVersionID: verificationVersion.ID,
		SubmissionURL:    validSubmissionURL,
		CreatedBy:        authorUserID,
		NamespaceSlug:    "seed-author",
		LibrarySlug:      "union-find",
		VersionNumber:    1,
		Visibility:       domain.LibraryVisibilityPublic,
		OwnerUserID:      authorUserID,
	}
}

// TestCreateLinkOnPublicLibraryStoresURLAndRegistrant is the core RED test: any
// authenticated user may register a plain absolute HTTP(S) URL on a public
// library version, and the ONLY write is the link row (no Submission/Outbox/S3).
func TestCreateLinkOnPublicLibraryStoresURLAndRegistrant(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)

	link, err := use.Create(context.Background(), verificationOtherActor, "seed-author", "union-find", 1, "  "+validSubmissionURL+"  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("repository Create calls = %d, want exactly 1 (no other write path)", repo.createCalls)
	}
	params := repo.createParams
	if params.SubmissionURL != validSubmissionURL {
		t.Fatalf("stored url = %q, want trimmed %q", params.SubmissionURL, validSubmissionURL)
	}
	if params.ActorUserID != verificationOtherActor.UserID {
		t.Fatalf("actor user id = %q", params.ActorUserID)
	}
	if params.LibraryVersionID != verificationVersion.ID {
		t.Fatalf("version id = %q", params.LibraryVersionID)
	}
	if params.NamespaceID == "" || params.LibraryID == "" {
		t.Fatalf("namespace/library ids not resolved: %+v", params)
	}
	if link == nil || link.SubmissionURL != validSubmissionURL || link.CreatedBy != verificationOtherActor.UserID {
		t.Fatalf("link = %+v", link)
	}
}

func TestCreateLinkRejectsInvalidURL(t *testing.T) {
	tooLong := "https://example.com/" + strings.Repeat("a", domain.MaxSubmissionURLLength)
	for _, raw := range []string{
		"", "   ", "not-a-url", "ftp://example.com/x", "https://",
		"https://?x", "https://host with space/x", "https:opaque",
		"https://example.com/\x00", tooLong,
	} {
		use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
		_, err := use.Create(context.Background(), verificationOtherActor, "seed-author", "union-find", 1, raw)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SUBMISSION_URL", verificationURLInvalidMessage)
		if repo.createCalls != 0 {
			t.Fatalf("repository must not be touched for invalid url %q", raw)
		}
	}
}

func TestCreateLinkAcceptsUnicodeHost(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	if _, err := use.Create(context.Background(), verificationOtherActor, "seed-author", "union-find", 1, "https://例え.jp/submissions/7"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createParams.SubmissionURL != "https://例え.jp/submissions/7" {
		t.Fatalf("stored url = %q", repo.createParams.SubmissionURL)
	}
}

func TestCreateLinkRequiresAuth(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	_, err := use.Create(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, validSubmissionURL)
	assertAppError(t, err, apperror.ErrUnauthorized, "UNAUTHORIZED", "authentication required")
	if repo.createCalls != 0 {
		t.Fatal("repository must not be touched for an anonymous registration")
	}
}

func TestCreateLinkPrivateHiddenFromNonOwner(t *testing.T) {
	for _, actor := range []auth.Actor{verificationOtherActor, adminActor()} {
		use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPrivate)
		_, err := use.Create(context.Background(), actor, "seed-author", "union-find", 1, validSubmissionURL)
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
		if repo.createCalls != 0 {
			t.Fatal("repository must not be touched for a hidden library")
		}
	}
}

func TestCreateLinkPrivateOwnerAllowed(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPrivate)
	if _, err := use.Create(context.Background(), authorActor(), "seed-author", "union-find", 1, validSubmissionURL); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("Create calls = %d", repo.createCalls)
	}
}

func TestCreateLinkValidatesVersionNumber(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	_, err := use.Create(context.Background(), verificationOtherActor, "seed-author", "union-find", 0, validSubmissionURL)
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	if repo.createCalls != 0 {
		t.Fatal("identifiers are validated before any side effect")
	}
}

func TestGetLinkPublicReadableAnonymously(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	link := publicLink()
	repo.byID = map[int64]*domain.LibraryVerification{15: &link}

	got, err := use.Get(context.Background(), auth.Actor{}, 15)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SubmissionURL != validSubmissionURL || got.CreatedBy != authorUserID {
		t.Fatalf("link = %+v", got)
	}
}

func TestGetLinkPrivateHiddenFromOthers(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	link := publicLink()
	link.Visibility = domain.LibraryVisibilityPrivate
	repo.byID = map[int64]*domain.LibraryVerification{15: &link}

	for _, actor := range []auth.Actor{auth.Actor{}, verificationOtherActor, adminActor()} {
		_, err := use.Get(context.Background(), actor, 15)
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
	got, err := use.Get(context.Background(), authorActor(), 15)
	if err != nil {
		t.Fatalf("owner Get: %v", err)
	}
	if got.SubmissionURL != validSubmissionURL {
		t.Fatalf("link = %+v", got)
	}
}

func TestGetLinkNotFound(t *testing.T) {
	use, _, _ := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	_, err := use.Get(context.Background(), authorActor(), 999)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
}

func TestGetLinkValidatesID(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	for _, id := range []int64{0, -1} {
		_, err := use.Get(context.Background(), authorActor(), id)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_LIBRARY_VERIFICATION_ID", "verification_id must be a positive integer")
	}
	if repo.findCalls != 0 {
		t.Fatal("repository must not be reached for an invalid id")
	}
}

func TestListLinksByVersion(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	link := publicLink()
	repo.listResult = []domain.LibraryVerification{link}

	links, err := use.List(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, 10, true, 3)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !repo.listCalled {
		t.Fatal("ListByVersion must be called")
	}
	if repo.listParams.LibraryVersionID != verificationVersion.ID {
		t.Fatalf("version id = %q", repo.listParams.LibraryVersionID)
	}
	if repo.listParams.Limit != 10 || repo.listParams.Offset != 3 {
		t.Fatalf("paging = %+v", repo.listParams)
	}
	if len(links) != 1 || links[0].SubmissionURL != validSubmissionURL {
		t.Fatalf("links = %+v", links)
	}
}

func TestListLinksPrivateHidden(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPrivate)
	_, err := use.List(context.Background(), verificationOtherActor, "seed-author", "union-find", 1, 10, true, 0)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if repo.listCalled {
		t.Fatal("ListByVersion must not be reached for a hidden library")
	}
}

func TestListLinksPagingOverflow(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)

	_, err := use.List(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, 10, true, -1)
	assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_OFFSET", "offset must be greater than or equal to 0")

	for _, offset := range []int{2147483648, 4294967297} {
		_, err := use.List(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, 10, true, offset)
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_OFFSET", "offset is too large")
	}
	if repo.listCalled {
		t.Fatal("ListByVersion must not be reached for an overflowing offset")
	}
	if _, err := use.List(context.Background(), auth.Actor{}, "seed-author", "union-find", 1, 10, true, 2147483647); err != nil {
		t.Fatalf("MaxInt32 offset must be accepted: %v", err)
	}
	if !repo.listCalled {
		t.Fatal("ListByVersion must be reached for a MaxInt32 offset")
	}
}

// TestListLinksThreadsActorIntoFetch is the RED-GREEN wiring regression for the
// Codex TOCTOU finding: the List usecase must pass the reader's user id into the
// repository fetch so the (now visibility-scoped) SQL statement can authorize
// the CURRENT library state. Before the fix, List called ListByVersion
// unconditionally with no actor id, so a public->private flip after Resolve let
// an anonymous/non-owner see the private links.
func TestListLinksThreadsActorIntoFetch(t *testing.T) {
	for _, actor := range []auth.Actor{
		{}, // anonymous
		verificationOtherActor,
		adminActor(),
		authorActor(),
	} {
		use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
		if _, err := use.List(context.Background(), actor, "seed-author", "union-find", 1, 10, true, 0); err != nil {
			t.Fatalf("List as %q: %v", actor.UserID, err)
		}
		if !repo.listCalled {
			t.Fatalf("actor %q: ListByVersion was not reached", actor.UserID)
		}
		if repo.listParams.ActorUserID != actor.UserID {
			t.Fatalf("actor %q: fetch ActorUserID = %q, want the reader's id", actor.UserID, repo.listParams.ActorUserID)
		}
	}
}

// TestGetLinkReevaluatesCurrentVisibilityPerFetch ensures the detail Get stays
// atomic: FindByID is a single statement whose joined row carries the library's
// CURRENT visibility, so the usecase must re-evaluate that snapshot on every
// read (not a stale cached decision). A link privatized between two reads is
// hidden immediately from every non-owner, including admins, while the owner
// still reads it.
func TestGetLinkReevaluatesCurrentVisibilityPerFetch(t *testing.T) {
	use, _, repo := newVerificationLinkUsecase(domain.LibraryVisibilityPublic)
	link := publicLink()
	repo.byID = map[int64]*domain.LibraryVerification{15: &link}

	// First fetch: public, readable anonymously.
	if _, err := use.Get(context.Background(), auth.Actor{}, 15); err != nil {
		t.Fatalf("public Get: %v", err)
	}
	// The library is privatized; the NEXT single-statement fetch returns the
	// CURRENT private visibility, so the same link must now be hidden.
	link.Visibility = domain.LibraryVisibilityPrivate
	for _, actor := range []auth.Actor{auth.Actor{}, verificationOtherActor, adminActor()} {
		_, err := use.Get(context.Background(), actor, 15)
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
	// The owner still reads it.
	if got, err := use.Get(context.Background(), authorActor(), 15); err != nil || got.SubmissionURL != validSubmissionURL {
		t.Fatalf("owner Get after privatization: link=%+v err=%v", got, err)
	}
}
