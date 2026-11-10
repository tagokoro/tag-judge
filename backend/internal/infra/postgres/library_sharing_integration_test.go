package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// assertAppError checks that err unwraps to sentinel and carries the shared
// API error contract: code and message are part of the design contract.
func assertAppError(t *testing.T, err error, sentinel error, wantCode, wantMessage string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is(err, %v) = false, err = %v", sentinel, err)
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

// seedLinkFixture creates one user (the namespace owner), namespace, library,
// and version. A submission URL link needs no Problem, Submission, or Outbox,
// so this fixture deliberately seeds none of them.
func seedLinkFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, visibility string) (namespaceID, libraryID, versionID, ownerUserID string) {
	t.Helper()
	ownerUserID = "00000000-0000-4000-8000-000000000001"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'link-owner', 'Owner', 'link-owner@example.test');`, ownerUserID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO namespaces (owner_user_id, slug, display_name)
VALUES ($1, 'link-author', 'Link Author');`, ownerUserID); err != nil {
		t.Fatalf("seed namespace: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT id::text FROM namespaces WHERE slug = 'link-author'`).Scan(&namespaceID); err != nil {
		t.Fatalf("read namespace: %v", err)
	}
	library, err := postgres.NewLibraryRepository(db).Create(ctx, repository.CreateLibraryParams{
		NamespaceID: namespaceID,
		Slug:        "union-find",
		Name:        "UnionFind",
		Visibility:  domain.LibraryVisibility(visibility),
	})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	source := "def add(a, b):\n    return a + b\n"
	version, err := postgres.NewLibraryVersionRepository(db).Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:     library.ID,
		SourceCode:    source,
		ContentSHA256: postgres.ContentSHA256(source),
		ByteSize:      len([]byte(source)),
		CreatedBy:     ownerUserID,
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	return namespaceID, library.ID, version.ID, ownerUserID
}

func linkParams(namespaceID, libraryID, versionID, actorUserID, submissionURL string) repository.CreateLibraryVerificationParams {
	return repository.CreateLibraryVerificationParams{
		ActorUserID:      actorUserID,
		NamespaceID:      namespaceID,
		LibraryID:        libraryID,
		LibraryVersionID: versionID,
		SubmissionURL:    submissionURL,
	}
}

func linkCount(t *testing.T, ctx context.Context, db *pgxpool.Pool) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM library_verifications`).Scan(&n); err != nil {
		t.Fatalf("count links: %v", err)
	}
	return n
}

// assertNoExecutionRows proves the URL-link create path wrote no Submission,
// Outbox, or code-run row. Those tables belong to the unchanged main
// submission/judge path; a submission URL link must never touch them.
func assertNoExecutionRows(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"submissions", "submission_job_outbox", "code_run_job_outbox"} {
		var got int64
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != 0 {
			t.Fatalf("%s count = %d, want 0 (URL link must not create execution rows)", table, got)
		}
	}
}

func assertCheckViolation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a check_violation, got nil")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a PostgreSQL error, got %v", err)
	}
	if pgErr.Code != "23514" {
		t.Fatalf("SQLSTATE = %q, want 23514 (check_violation)", pgErr.Code)
	}
	if !strings.HasPrefix(pgErr.ConstraintName, "library_verifications_") {
		t.Fatalf("constraint = %q, want a library_verifications constraint", pgErr.ConstraintName)
	}
}

// TestNamespaceUniqueConstraints verifies that the PostgreSQL repository maps
// the unique constraints on namespaces to their API conflict contracts.
func TestNamespaceUniqueConstraints(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('00000000-0000-4000-8000-000000000001', 'test', 'ns-owner', 'Owner', 'ns-owner@example.test'),
       ('00000000-0000-4000-8000-000000000002', 'test', 'ns-admin', 'Admin', 'ns-admin@example.test');
`); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	repo := postgres.NewNamespaceRepository(db)
	if _, err := repo.Create(ctx, repository.CreateNamespaceParams{
		OwnerUserID: "00000000-0000-4000-8000-000000000001",
		Slug:        "seed-author",
		DisplayName: "Seed Author",
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateNamespaceParams{
		OwnerUserID: "00000000-0000-4000-8000-000000000001",
		Slug:        "other-slug",
		DisplayName: "Other",
	})
	assertAppError(t, err, apperror.ErrConflict, "NAMESPACE_ALREADY_EXISTS", "user already has a namespace")

	_, err = repo.Create(ctx, repository.CreateNamespaceParams{
		OwnerUserID: "00000000-0000-4000-8000-000000000002",
		Slug:        "seed-author",
		DisplayName: "Copycat",
	})
	assertAppError(t, err, apperror.ErrConflict, "NAMESPACE_SLUG_TAKEN", "namespace slug is already taken")
}

// Uses an isolated schema and the real migrations. Set
// SUBMISSION_DETAILS_TEST_DATABASE_URL to a local test database.
func TestLibraryVersionIsImmutable(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)

	source := "def add(a, b):\n    return a + b\n"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('00000000-0000-4000-8000-000000000001', 'test', 'library', 'Library', 'library@example.test');
INSERT INTO namespaces (owner_user_id, slug, display_name)
VALUES ('00000000-0000-4000-8000-000000000001', 'seed-author', 'Seed Author');
INSERT INTO libraries (namespace_id, slug, name, description, visibility)
VALUES ((SELECT id FROM namespaces WHERE slug = 'seed-author'), 'union-find', 'UnionFind', '', 'private');
`); err != nil {
		t.Fatalf("seed library sharing tables: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO library_versions (library_id, version_number, source_code, content_sha256, byte_size, created_by)
VALUES ((SELECT id FROM libraries WHERE slug = 'union-find'), 1, $1,
        encode(digest(convert_to($1, 'UTF8'), 'sha256'), 'hex'), octet_length($1),
        '00000000-0000-4000-8000-000000000001');
`, source); err != nil {
		t.Fatalf("seed library version: %v", err)
	}

	if _, err := db.Exec(ctx, `UPDATE library_versions SET source_code = source_code || '# mutated' WHERE version_number = 1`); err == nil {
		t.Fatal("UPDATE library_versions must be rejected")
	} else if !strings.Contains(err.Error(), "library_versions are immutable") {
		t.Fatalf("UPDATE must fail with the immutability trigger, got: %v", err)
	}

	// byte_size must match the stored UTF-8 bytes exactly.
	if _, err := db.Exec(ctx, `
INSERT INTO library_versions (library_id, version_number, source_code, content_sha256, byte_size, created_by)
VALUES ((SELECT id FROM libraries WHERE slug = 'union-find'), 2,
        'def same(a, b): return a + b' || chr(10),
        encode(digest(convert_to('def same(a, b): return a + b' || chr(10), 'UTF8'), 'sha256'), 'hex'),
        octet_length('def same(a, b): return a + b' || chr(10)) + 1,
        '00000000-0000-4000-8000-000000000001')`); err == nil {
		t.Fatal("byte_size mismatch must be rejected by the DB constraint")
	}

	// content_sha256 must equal the hash of the stored source bytes.
	if _, err := db.Exec(ctx, `
INSERT INTO library_versions (library_id, version_number, source_code, content_sha256, byte_size, created_by)
VALUES ((SELECT id FROM libraries WHERE slug = 'union-find'), 2,
        'def same(a, b): return a + b' || chr(10),
        repeat('0', 64),
        octet_length('def same(a, b): return a + b' || chr(10)),
        '00000000-0000-4000-8000-000000000001')`); err == nil {
		t.Fatal("content_sha256 mismatch must be rejected by the DB constraint")
	}

	// DELETE must also be rejected by the trigger.
	if _, err := db.Exec(ctx, `DELETE FROM library_versions WHERE version_number = 1`); err == nil {
		t.Fatal("DELETE library_versions must be rejected")
	} else if !strings.Contains(err.Error(), "library_versions are immutable") {
		t.Fatalf("DELETE must fail with the immutability trigger, got: %v", err)
	}
}

// TestCreateLibraryVersionAllocatesConsecutiveNumbersConcurrently starts two
// version creations for the same library at the same time and expects distinct
// consecutive numbers from the FOR UPDATE allocation query alone.
func TestCreateLibraryVersionAllocatesConsecutiveNumbersConcurrently(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('00000000-0000-4000-8000-000000000001', 'test', 'version-owner', 'Owner', 'version-owner@example.test');
INSERT INTO namespaces (owner_user_id, slug, display_name)
VALUES ('00000000-0000-4000-8000-000000000001', 'version-owner', 'Version Owner');
`); err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	var namespaceID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM namespaces WHERE slug = 'version-owner'`).Scan(&namespaceID); err != nil {
		t.Fatalf("find namespace: %v", err)
	}

	library, err := postgres.NewLibraryRepository(db).Create(ctx, repository.CreateLibraryParams{
		NamespaceID: namespaceID,
		Slug:        "union-find",
		Name:        "UnionFind",
		Visibility:  domain.LibraryVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	versions := postgres.NewLibraryVersionRepository(db)
	start := make(chan struct{})
	const workers = 8
	results := make([]string, workers)
	numbers := make([]int, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			userID := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
			if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'version-worker-' || $2, 'Worker', 'worker@example.test')
ON CONFLICT DO NOTHING`, userID, fmt.Sprint(i+1)); err != nil {
				errs[i] = err
				return
			}
			version, err := versions.Create(ctx, repository.CreateLibraryVersionParams{
				LibraryID:     library.ID,
				SourceCode:    fmt.Sprintf("# worker %d\nx = %d\n", i+1, i+1),
				ContentSHA256: postgres.ContentSHA256(fmt.Sprintf("# worker %d\nx = %d\n", i+1, i+1)),
				ByteSize:      len([]byte(fmt.Sprintf("# worker %d\nx = %d\n", i+1, i+1))),
				CreatedBy:     userID,
			})
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = version.ID
			numbers[i] = version.VersionNumber
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d create: %v", i, err)
		}
	}
	seenID := make(map[string]bool, workers)
	seenNumber := make(map[int]bool, workers)
	for i := range workers {
		if seenID[results[i]] {
			t.Fatalf("duplicated version id %s", results[i])
		}
		if seenNumber[numbers[i]] {
			t.Fatalf("duplicated version number %d", numbers[i])
		}
		seenID[results[i]] = true
		seenNumber[numbers[i]] = true
	}
	for want := 1; want <= workers; want++ {
		if !seenNumber[want] {
			t.Fatalf("consecutive numbering broken: got numbers %v", numbers)
		}
	}
}

// TestCreateLibraryVerificationStoresURLAndRegistrant proves a create stores
// only the absolute submission URL + registrant + version + timestamp, returns
// the current library state, and creates no Submission/Outbox/CodeRun rows.
func TestCreateLibraryVerificationStoresURLAndRegistrant(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")
	repo := postgres.NewLibraryVerificationRepository(db)

	const url = "https://atcoder.jp/contests/abc001/submissions/1234567"
	link, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, ownerUserID, url))
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if link.ID <= 0 {
		t.Fatalf("id = %d, want positive", link.ID)
	}
	if link.LibraryVersionID != versionID {
		t.Fatalf("library_version_id = %q, want %q", link.LibraryVersionID, versionID)
	}
	if link.SubmissionURL != url {
		t.Fatalf("submission_url = %q, want %q", link.SubmissionURL, url)
	}
	if link.CreatedBy != ownerUserID {
		t.Fatalf("created_by = %q, want %q", link.CreatedBy, ownerUserID)
	}
	if link.CreatedAt.IsZero() {
		t.Fatal("created_at is zero")
	}
	if link.Visibility != domain.LibraryVisibilityPublic {
		t.Fatalf("visibility = %q, want public", link.Visibility)
	}
	if link.OwnerUserID != ownerUserID {
		t.Fatalf("owner_user_id = %q, want %q", link.OwnerUserID, ownerUserID)
	}
	if link.NamespaceSlug != "link-author" {
		t.Fatalf("namespace_slug = %q, want link-author", link.NamespaceSlug)
	}
	if link.LibrarySlug != "union-find" {
		t.Fatalf("library_slug = %q, want union-find", link.LibrarySlug)
	}
	if link.VersionNumber != 1 {
		t.Fatalf("version_number = %d, want 1", link.VersionNumber)
	}

	// The persisted row has only the URL, registrant, version and timestamp;
	// the old submission_id / adapter_source / adapter_sha256 columns are gone.
	var gotURL, gotCreatedBy string
	if err := db.QueryRow(ctx, `SELECT submission_url, created_by::text FROM library_verifications WHERE id = $1`, link.ID).Scan(&gotURL, &gotCreatedBy); err != nil {
		t.Fatalf("read link row: %v", err)
	}
	if gotURL != url || gotCreatedBy != ownerUserID {
		t.Fatalf("link row url=%q created_by=%q", gotURL, gotCreatedBy)
	}
	var columnCount int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'library_verifications' AND column_name IN ('submission_id','adapter_source','adapter_sha256')`).Scan(&columnCount); err != nil {
		t.Fatalf("check removed columns: %v", err)
	}
	if columnCount != 0 {
		t.Fatalf("adapter/submission columns still present: %d", columnCount)
	}

	assertNoExecutionRows(t, ctx, db)

	// FindByID returns the same current-state link.
	detail, err := repo.FindByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("find link: %v", err)
	}
	if detail.ID != link.ID || detail.SubmissionURL != url || detail.CreatedBy != ownerUserID ||
		detail.LibraryVersionID != versionID || detail.Visibility != domain.LibraryVisibilityPublic ||
		detail.OwnerUserID != ownerUserID {
		t.Fatalf("FindByID does not match Create: create=%+v detail=%+v", link, detail)
	}

	_, err = repo.FindByID(ctx, link.ID+999999)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
}

// TestCreateLibraryVerificationAllowsMultipleURLsAndListsByVersion proves
// several self-declared URLs may be attached to one version, that ListByVersion
// returns them newest-first with pagination, and that none create executions.
func TestCreateLibraryVerificationAllowsMultipleURLsAndListsByVersion(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")
	repo := postgres.NewLibraryVerificationRepository(db)

	urls := []string{"https://example.com/submissions/1", "https://example.com/submissions/2", "https://example.com/submissions/3"}
	for _, u := range urls {
		if _, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, ownerUserID, u)); err != nil {
			t.Fatalf("create link %q: %v", u, err)
		}
	}

	links, err := repo.ListByVersion(ctx, repository.ListLibraryVerificationsParams{LibraryVersionID: versionID, Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != len(urls) {
		t.Fatalf("list len = %d, want %d", len(links), len(urls))
	}
	got := map[string]bool{}
	for i, link := range links {
		got[link.SubmissionURL] = true
		if link.CreatedBy != ownerUserID || link.LibraryVersionID != versionID {
			t.Fatalf("link %d = %+v", i, link)
		}
		if i > 0 && links[i-1].ID <= link.ID {
			t.Fatalf("links must be ordered by id DESC: %v", links)
		}
	}
	for _, u := range urls {
		if !got[u] {
			t.Fatalf("missing URL %q in list", u)
		}
	}

	page, err := repo.ListByVersion(ctx, repository.ListLibraryVerificationsParams{LibraryVersionID: versionID, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("page len = %d, want 1", len(page))
	}

	assertNoExecutionRows(t, ctx, db)
}

// TestCreateLibraryVerificationRejectsPrivateAndForeignVersion proves a failed
// re-check (private library for a non-owner, foreign version) commits nothing.
func TestCreateLibraryVerificationRejectsPrivateAndForeignVersion(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")

	stranger := "00000000-0000-4000-8000-000000000002"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'link-stranger', 'Stranger', 'stranger@example.test');`, stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}
	repo := postgres.NewLibraryVerificationRepository(db)

	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}
	_, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, stranger, "https://example.com/private"))
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if linkCount(t, ctx, db) != 0 {
		t.Fatal("private rejection leaked a link row")
	}
	assertNoExecutionRows(t, ctx, db)

	// A version id outside of the library fails the same way.
	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'public' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("restore visibility: %v", err)
	}
	_, err = repo.Create(ctx, linkParams(namespaceID, libraryID, "11111111-1111-4111-8111-111111111111", ownerUserID, "https://example.com/foreign"))
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if linkCount(t, ctx, db) != 0 {
		t.Fatal("foreign version rejection leaked a link row")
	}
	assertNoExecutionRows(t, ctx, db)
}

// TestCreateLibraryVerificationRechecksVisibilityInTransaction proves the
// FOR SHARE re-check: a non-owner cannot add a link once the library is private,
// while the owner still can. No re-check commits any execution rows.
func TestCreateLibraryVerificationRechecksVisibilityInTransaction(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")

	stranger := "00000000-0000-4000-8000-000000000003"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'link-stranger', 'Stranger', 'stranger@example.test');`, stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}
	repo := postgres.NewLibraryVerificationRepository(db)

	// Public: a stranger may add a link.
	if _, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, stranger, "https://example.com/public")); err != nil {
		t.Fatalf("public link: %v", err)
	}

	// Owner flips private; the same stranger now commits nothing.
	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}
	before := linkCount(t, ctx, db)
	_, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, stranger, "https://example.com/private"))
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	after := linkCount(t, ctx, db)
	if before != 1 || after != 1 {
		t.Fatalf("private link must commit nothing: before=%d after=%d", before, after)
	}

	// The owner may still add a link to their own private library.
	if _, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, ownerUserID, "https://example.com/owner")); err != nil {
		t.Fatalf("owner private link: %v", err)
	}
	if linkCount(t, ctx, db) != 2 {
		t.Fatalf("owner link was not committed")
	}
	assertNoExecutionRows(t, ctx, db)
}

// TestCreateLibraryVerificationVisibilityTOCTOU uses two independent
// connections and a barrier: the visibility UPDATE holds its row lock while
// link creation starts, then commits. The create must observe the committed
// private state rather than a stale pre-lock snapshot, and commit nothing.
func TestCreateLibraryVerificationVisibilityTOCTOU(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, _ := seedLinkFixture(t, ctx, db, "public")
	stranger := "00000000-0000-4000-8000-000000000003"
	if _, err := db.Exec(ctx, `INSERT INTO users (id, provider, provider_user_id, name, email) VALUES ($1, 'test', 'toctou-stranger', 'Stranger', 'toctou@example.test')`, stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}

	// A dedicated one-connection pool so pg_stat_activity can identify the
	// create transaction by application_name. The observation barrier is the
	// database's Lock wait, not merely goroutine scheduling.
	var schema string
	if err := db.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("read test schema: %v", err)
	}
	workerConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse worker database URL: %v", err)
	}
	workerConfig.MaxConns = 1
	workerConfig.MinConns = 1
	workerConfig.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	workerApplicationName := "tag-judge-toctou-worker"
	workerConfig.ConnConfig.RuntimeParams["application_name"] = workerApplicationName
	workerDB, err := pgxpool.NewWithConfig(ctx, workerConfig)
	if err != nil {
		t.Fatalf("open worker database: %v", err)
	}
	defer workerDB.Close()
	workerRepo := postgres.NewLibraryVerificationRepository(workerDB)

	conn, err := db.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire visibility connection: %v", err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin visibility transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("stage visibility update: %v", err)
	}
	baselineLinks := linkCount(t, ctx, db)

	verificationStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(verificationStarted)
		_, verifyErr := workerRepo.Create(ctx, linkParams(namespaceID, libraryID, versionID, stranger, "https://example.com/toctou"))
		result <- verifyErr
	}()
	<-verificationStarted
	lockWaitDeadline := time.Now().Add(15 * time.Second)
	observedLockWait := false
	for time.Now().Before(lockWaitDeadline) {
		var waitEventType, query string
		err := db.QueryRow(ctx, `
SELECT wait_event_type, query
FROM pg_stat_activity
WHERE application_name = $1
  AND state = 'active'
  AND wait_event_type = 'Lock'
  AND query LIKE '%LockLibraryForVerification%'
LIMIT 1`, workerApplicationName).Scan(&waitEventType, &query)
		if err == nil {
			if !strings.Contains(query, "LockLibraryForVerification") || !strings.Contains(strings.ToUpper(query), "FOR SHARE") {
				t.Fatalf("observed unexpected create lock query: %q", query)
			}
			if waitEventType != "Lock" {
				t.Fatalf("create wait_event_type = %q, want Lock", waitEventType)
			}
			observedLockWait = true
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("observe create lock wait: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observedLockWait {
		_ = tx.Rollback(ctx)
		<-result
		t.Fatal("timed out waiting to observe LockLibraryForVerification FOR SHARE lock wait")
	}
	// Without FOR SHARE this ordering is not guaranteed: the creator could
	// read public before the updater commits and successfully insert a link.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit visibility update: %v", err)
	}
	if err := <-result; err == nil {
		t.Fatal("link creation succeeded despite concurrent visibility change")
	} else {
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
	if got := linkCount(t, ctx, db); got != baselineLinks {
		t.Fatalf("library_verifications count = %d, want %d after rejected create", got, baselineLinks)
	}
	assertNoExecutionRows(t, ctx, db)
}

// TestLibraryVerificationSubmissionURLConstraints proves the DB backstops the
// shared URL rules: only absolute HTTP(S) URLs with a host, no control chars,
// and at most 2048 bytes are stored, and a rejected create commits nothing.
func TestLibraryVerificationSubmissionURLConstraints(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")

	insert := func(ctx context.Context, url string) error {
		_, err := db.Exec(ctx, `
INSERT INTO library_verifications (library_version_id, created_by, submission_url)
VALUES ($1::uuid, $2::uuid, $3)`, versionID, ownerUserID, url)
		return err
	}

	// A 2048-byte URL is allowed; 2049 bytes is rejected.
	prefix := "https://e.com/"
	valid2048 := prefix + strings.Repeat("a", 2048-len(prefix))
	invalid2049 := prefix + strings.Repeat("a", 2049-len(prefix))
	if err := insert(ctx, valid2048); err != nil {
		t.Fatalf("2048-byte URL must be accepted: %v", err)
	}

	invalid := []struct {
		name string
		url  string
	}{
		{"scheme only", "https://"},
		{"hostless", "not-a-url"},
		{"wrong scheme", "ftp://example.com/x"},
		{"control char", "https://example.com/\x07"},
		{"too long", invalid2049},
	}
	for _, tc := range invalid {
		assertCheckViolation(t, insert(ctx, tc.url))
	}

	// repository.Create with a bad URL also fails at the INSERT and commits
	// nothing (the transaction rolls back), leaving only the valid row.
	repo := postgres.NewLibraryVerificationRepository(db)
	if _, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, ownerUserID, "https://")); err == nil {
		t.Fatal("Create with a scheme-only URL must fail")
	}
	if got := linkCount(t, ctx, db); got != 1 {
		t.Fatalf("library_verifications count = %d, want 1 after rejected create", got)
	}
	assertNoExecutionRows(t, ctx, db)
}

// TestLibraryVerificationReadsReflectCurrentVisibility proves read methods
// return the library's CURRENT visibility/owner so the API layer can authorize
// private links by the owner only, and that visibility changes are followed.
func TestLibraryVerificationReadsReflectCurrentVisibility(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedLinkFixture(t, ctx, db, "public")
	repo := postgres.NewLibraryVerificationRepository(db)

	link, err := repo.Create(ctx, linkParams(namespaceID, libraryID, versionID, ownerUserID, "https://example.com/sub/1"))
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if link.Visibility != domain.LibraryVisibilityPublic || link.OwnerUserID != ownerUserID {
		t.Fatalf("public read context = %+v", link)
	}

	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}
	detail, err := repo.FindByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("find link after privatization: %v", err)
	}
	if detail.Visibility != domain.LibraryVisibilityPrivate || detail.OwnerUserID != ownerUserID {
		t.Fatalf("private read context = %+v", detail)
	}
	links, err := repo.ListByVersion(ctx, repository.ListLibraryVerificationsParams{LibraryVersionID: versionID, Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("list after privatization: %v", err)
	}
	if len(links) != 1 || links[0].Visibility != domain.LibraryVisibilityPrivate {
		t.Fatalf("list read context = %+v", links)
	}
	assertNoExecutionRows(t, ctx, db)
}
