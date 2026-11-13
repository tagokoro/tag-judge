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

// seedVersionFixture creates one user (the category owner), category, library,
// and version. A submission URL needs no Problem, Submission, or Outbox, so
// this fixture deliberately seeds none of them.
func seedVersionFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, visibility string) (categoryID, libraryID, versionID, ownerUserID string) {
	t.Helper()
	ownerUserID = "00000000-0000-4000-8000-000000000001"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'link-owner', 'Owner', 'link-owner@example.test');`, ownerUserID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO user_profiles (user_id, handle) VALUES ($1, 'link-owner');`, ownerUserID); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO categories (owner_user_id, name)
VALUES ($1, 'Link Author');`, ownerUserID); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT id::text FROM categories WHERE name = 'Link Author'`).Scan(&categoryID); err != nil {
		t.Fatalf("read category: %v", err)
	}
	library, err := postgres.NewLibraryRepository(db).Create(ctx, repository.CreateLibraryParams{
		OwnerUserID:  ownerUserID,
		CategoryName: "Link Author",
		Name:         "UnionFind",
		Visibility:   domain.LibraryVisibility(visibility),
	})
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	source := "def add(a, b):\n    return a + b\n"
	version, err := postgres.NewLibraryVersionRepository(db).Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:  library.ID,
		SourceCode: source,
		CreatedBy:  ownerUserID,
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	return categoryID, library.ID, version.ID, ownerUserID
}

// assertNoExecutionRows proves the submission-URL path wrote no Submission,
// Outbox, or code-run row. Those tables belong to the unchanged main
// submission/judge path; a submission URL must never touch them.
func assertNoExecutionRows(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"submissions", "submission_job_outbox", "code_run_job_outbox"} {
		var got int64
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != 0 {
			t.Fatalf("%s count = %d, want 0 (submission URL must not create execution rows)", table, got)
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
	if !strings.HasPrefix(pgErr.ConstraintName, "library_versions_submission_url_") {
		t.Fatalf("constraint = %q, want a library_versions submission_url constraint", pgErr.ConstraintName)
	}
}

// invalidSubmissionURLs must be rejected by BOTH the DB CHECK backstop (raw
// SQL) and the authoritative domain/repository validator. The first three are
// the hostless authorities the original "^https?://[^/[:space:][:cntrl:]]+"
// CHECK wrongly accepted (Codex finding): "?x", "@" and ":443" have no host.
// The userinfo-only/no-host cases ("https://user@", "https://user@/",
// "https://user@?x", ...) are the raw DB regression that the prefix CHECK
// wrongly accepted and that the anchored CHECK must now reject. Whitespace,
// control and no-host cases are included as regressions.
var invalidSubmissionURLs = []struct{ name, url string }{
	{"hostless query", "https://?x"},
	{"hostless at", "https://@"},
	{"hostless port", "https://:443"},
	{"hostless fragment", "https://#frag"},
	{"hostless path", "https:///path"},
	{"scheme only https", "https://"},
	{"scheme only http", "http://"},
	{"no scheme", "not-a-url"},
	{"wrong scheme", "ftp://example.com/x"},
	{"opaque", "https:example.com"},
	{"userinfo no host", "https://user@"},
	{"userinfo no host trailing slash", "https://user@/"},
	{"userinfo no host query", "https://user@?x"},
	{"userinfo no host fragment", "https://user@#frag"},
	{"userinfo password no host", "https://user:pass@"},
	{"userinfo password no host path", "https://user:pass@/path"},
	{"empty userinfo no host path", "https://@/"},
	{"empty userinfo no host query", "https://@?x"},
	{"control char", "https://example.com/\x07"},
	{"internal space", "https://exa mple.com"},
	{"too long", "https://e.com/" + strings.Repeat("a", 2049-len("https://e.com/"))},
}

// validSubmissionURLs are representative absolute HTTP(S) URLs with a real
// host, all of which must be accepted by BOTH layers. They cover localhost with
// a port, query, fragment, IPv4, IPv6 literals, the 2048-byte limit, userinfo
// with a real host, and INTERNATIONALIZED (Unicode/IDN) hosts - the spec says
// any absolute HTTP(S) URL with a host, so no ASCII-only host rule is allowed.
// "userinfo containing @" guards that the DB backstop stays a subset of the
// authoritative net/url Hostname() check (net/url resolves the LAST @ as the
// userinfo/host separator, so "https://a@b@c" has host "c" and must be kept).
var validSubmissionURLs = []struct{ name, url string }{
	{"simple host", "https://example.com"},
	{"submission path", "https://example.com/submissions/12345"},
	{"localhost port path query", "http://localhost:3000/submit?q=1"},
	{"ipv4 port", "http://127.0.0.1:8080/x"},
	{"ipv6 literal port", "http://[::1]:8080/path"},
	{"query no path", "https://example.com?q=1"},
	{"fragment", "https://example.com#frag"},
	{"localhost", "http://localhost"},
	{"port path query fragment", "https://example.com:8443/x?y=2#z"},
	{"userinfo with host", "https://user@example.com/sub"},
	{"empty userinfo with host", "https://@example.com"},
	{"userinfo containing @", "https://a@b@c"},
	{"unicode host", "https://例え.jp/submissions/1"},
	{"unicode host with query", "https://例え.jp?q=1"},
	{"unicode host with port", "http://日本.jp:8080/x"},
	{"punycode idn host", "https://xn--r8jz45g.jp/sub/1"},
	{"max length 2048", "https://e.com/" + strings.Repeat("a", 2048-len("https://e.com/"))},
}

// TestCategoryRepositoryRules verifies how the PostgreSQL repositories keep
// categories: a library's category is found by name ignoring case or created
// with it, a handle is required, the per-user limit holds, ids are random
// 20-hex public ids, visibility follows public libraries, and a move creates
// the target and removes the category it leaves empty.
func TestCategoryRepositoryRules(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	const (
		owner  = "00000000-0000-4000-8000-000000000001"
		other  = "00000000-0000-4000-8000-000000000002"
		noName = "00000000-0000-4000-8000-000000000003"
	)
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('`+owner+`', 'test', 'cat-owner', 'Owner', 'cat-owner@example.test'),
       ('`+other+`', 'test', 'cat-other', 'Other', 'cat-other@example.test'),
       ('`+noName+`', 'test', 'cat-none', 'None', 'cat-none@example.test');
INSERT INTO user_profiles (user_id, handle)
VALUES ('`+owner+`', 'owner'),
       ('`+other+`', 'other');
`); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	categories := postgres.NewCategoryRepository(db)
	libraries := postgres.NewLibraryRepository(db)
	create := func(ownerUserID, category, name string, visibility domain.LibraryVisibility) (*domain.Library, error) {
		return libraries.Create(ctx, repository.CreateLibraryParams{
			OwnerUserID: ownerUserID, CategoryName: category, Name: name, Visibility: visibility,
		})
	}
	countCategories := func(ownerUserID string) int {
		t.Helper()
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM categories WHERE owner_user_id = $1`, ownerUserID).Scan(&n); err != nil {
			t.Fatalf("count categories: %v", err)
		}
		return n
	}

	_, err := create(noName, "グラフ", "Dijkstra", domain.LibraryVisibilityPublic)
	assertAppError(t, err, apperror.ErrConflict, "HANDLE_REQUIRED", "create a handle before creating a library")
	if countCategories(noName) != 0 {
		t.Fatal("a rejected create must not leave a category behind")
	}

	// The first library creates the category; a second one with the same name
	// in another letter case reuses it.
	dijkstra, err := create(owner, "Graph", "Dijkstra", domain.LibraryVisibilityPrivate)
	if err != nil {
		t.Fatalf("create private library: %v", err)
	}
	if !domain.ValidPublicID(dijkstra.PublicID) || !domain.ValidPublicID(dijkstra.CategoryPublicID) ||
		dijkstra.CategoryName != "Graph" || dijkstra.OwnerHandle != "owner" || dijkstra.OwnerUserID != owner {
		t.Fatalf("library = %+v", dijkstra)
	}
	bfs, err := create(owner, "graph", "BFS", domain.LibraryVisibilityPublic)
	if err != nil {
		t.Fatalf("create public library: %v", err)
	}
	if bfs.CategoryPublicID != dijkstra.CategoryPublicID || bfs.CategoryName != "Graph" || countCategories(owner) != 1 {
		t.Fatalf("bfs = %+v, want it in the existing Graph category", bfs)
	}
	if _, err := create(owner, "Graph", "Dijkstra", domain.LibraryVisibilityPrivate); err != nil {
		t.Fatalf("library names may repeat: %v", err)
	}
	if _, err := create(other, "Graph", "Theirs", domain.LibraryVisibilityPublic); err != nil {
		t.Fatalf("another owner reusing the category name: %v", err)
	}

	// Visibility follows the category's public libraries.
	graph, err := categories.FindByPublicID(ctx, dijkstra.CategoryPublicID)
	if err != nil || !graph.HasPublicLibrary || graph.Name != "Graph" {
		t.Fatalf("graph = %+v, %v; want it visible through BFS", graph, err)
	}
	math, err := create(owner, "数学", "Sieve", domain.LibraryVisibilityPrivate)
	if err != nil {
		t.Fatalf("create math library: %v", err)
	}
	visible, err := categories.ListByOwner(ctx, repository.ListCategoriesParams{OwnerUserID: owner, Limit: 100})
	if err != nil || len(visible) != 1 || visible[0].PublicID != graph.PublicID {
		t.Fatalf("categories visible to others = %+v, %v; want only Graph", visible, err)
	}
	all, err := categories.ListByOwner(ctx, repository.ListCategoriesParams{OwnerUserID: owner, IncludeHidden: true, Limit: 100})
	if err != nil || len(all) != 2 || all[0].PublicID != graph.PublicID {
		t.Fatalf("owner's categories = %+v, %v; want Graph then 数学", all, err)
	}
	listed, err := libraries.ListByCategory(ctx, repository.ListLibrariesParams{CategoryID: graph.ID, Limit: 100})
	if err != nil || len(listed) != 1 || listed[0].Name != "BFS" {
		t.Fatalf("public libraries = %+v, %v; want only BFS", listed, err)
	}
	listed, err = libraries.ListByCategory(ctx, repository.ListLibrariesParams{CategoryID: graph.ID, IncludePrivate: true, Limit: 100})
	if err != nil || len(listed) != 3 {
		t.Fatalf("all libraries = %+v, %v; want 3", listed, err)
	}

	// Renaming keeps the id; a rename onto another of the owner's names conflicts.
	description := "graphs and trees"
	newName := "グラフ"
	updated, err := categories.Update(ctx, repository.UpdateCategoryParams{ID: graph.ID, Name: &newName, Description: &description})
	if err != nil || updated.Description != description || updated.Name != "グラフ" || updated.PublicID != graph.PublicID {
		t.Fatalf("Update = %+v, %v", updated, err)
	}
	clash := "数学"
	_, err = categories.Update(ctx, repository.UpdateCategoryParams{ID: graph.ID, Name: &clash})
	assertAppError(t, err, apperror.ErrConflict, "CATEGORY_NAME_TAKEN", "you already have a category with this name")

	// Moving the only library out of 数学 into a new category creates that
	// category, keeps the library's id, and removes the emptied 数学.
	target := "整数論"
	moved, err := libraries.Update(ctx, repository.UpdateLibraryParams{ID: math.ID, OwnerUserID: owner, CategoryName: &target})
	if err != nil || moved.PublicID != math.PublicID || moved.CategoryName != "整数論" || moved.CategoryPublicID == math.CategoryPublicID {
		t.Fatalf("moved = %+v, %v", moved, err)
	}
	if _, err := categories.FindByPublicID(ctx, math.CategoryPublicID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("emptied category err = %v, want it removed", err)
	}
	// Moving into a category that still has other libraries keeps the source.
	back := "グラフ"
	if _, err := libraries.Update(ctx, repository.UpdateLibraryParams{ID: dijkstra.ID, OwnerUserID: owner, CategoryName: &target}); err != nil {
		t.Fatalf("move dijkstra: %v", err)
	}
	if _, err := categories.FindByPublicID(ctx, graph.PublicID); err != nil {
		t.Fatalf("a category with libraries left must stay: %v", err)
	}
	// Moving within the same category (by any letter case) changes nothing.
	same, err := libraries.Update(ctx, repository.UpdateLibraryParams{ID: bfs.ID, OwnerUserID: owner, CategoryName: &back})
	if err != nil || same.CategoryPublicID != graph.PublicID {
		t.Fatalf("same-category move = %+v, %v", same, err)
	}

	// The per-user limit counts categories, not libraries.
	for i := countCategories(owner); i < domain.MaxCategoriesPerUser; i++ {
		if _, err := create(owner, fmt.Sprintf("cat-%d", i), "L", domain.LibraryVisibilityPrivate); err != nil {
			t.Fatalf("category %d: %v", i, err)
		}
	}
	_, err = create(owner, "one-too-many", "L", domain.LibraryVisibilityPrivate)
	assertAppError(t, err, apperror.ErrConflict, "CATEGORY_LIMIT", "a user can own at most 20 categories")
	if _, err := create(owner, "グラフ", "Still fits", domain.LibraryVisibilityPrivate); err != nil {
		t.Fatalf("an existing category is usable at the limit: %v", err)
	}
	full := "one-too-many"
	_, err = libraries.Update(ctx, repository.UpdateLibraryParams{ID: bfs.ID, OwnerUserID: owner, CategoryName: &full})
	assertAppError(t, err, apperror.ErrConflict, "CATEGORY_LIMIT", "a user can own at most 20 categories")
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
INSERT INTO user_profiles (user_id, handle)
VALUES ('00000000-0000-4000-8000-000000000001', 'library');
INSERT INTO categories (owner_user_id, name)
VALUES ('00000000-0000-4000-8000-000000000001', 'Seed Author');
INSERT INTO libraries (category_id, name, description, visibility)
VALUES ((SELECT id FROM categories WHERE name = 'Seed Author'), 'UnionFind', '', 'private');
`); err != nil {
		t.Fatalf("seed library sharing tables: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO library_versions (library_id, version_number, source_code, content_sha256, byte_size, created_by)
VALUES ((SELECT id FROM libraries WHERE name = 'UnionFind'), 1, $1,
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
VALUES ((SELECT id FROM libraries WHERE name = 'UnionFind'), 2,
        'def same(a, b): return a + b' || chr(10),
        encode(digest(convert_to('def same(a, b): return a + b' || chr(10), 'UTF8'), 'sha256'), 'hex'),
        octet_length('def same(a, b): return a + b' || chr(10)) + 1,
        '00000000-0000-4000-8000-000000000001')`); err == nil {
		t.Fatal("byte_size mismatch must be rejected by the DB constraint")
	}

	// content_sha256 must equal the hash of the stored source bytes.
	if _, err := db.Exec(ctx, `
INSERT INTO library_versions (library_id, version_number, source_code, content_sha256, byte_size, created_by)
VALUES ((SELECT id FROM libraries WHERE name = 'UnionFind'), 2,
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

func TestCreateLibraryVersionMissingLibraryReturnsNotFound(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, err := postgres.NewLibraryVersionRepository(db).Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:  "00000000-0000-4000-8000-000000000099",
		SourceCode: "print(1)\n",
		CreatedBy:  "00000000-0000-4000-8000-000000000001",
	})
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
}

func TestCreateLibraryVersionDerivesDigestAndByteSize(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, libraryID, _, ownerUserID := seedVersionFixture(t, ctx, db, "public")
	source := "// 日本語\n"
	version, err := postgres.NewLibraryVersionRepository(db).Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:  libraryID,
		SourceCode: source,
		CreatedBy:  ownerUserID,
	})
	if err != nil {
		t.Fatalf("create with derived metadata: %v", err)
	}
	if version.ContentSHA256 != postgres.ContentSHA256(source) || version.ByteSize != len([]byte(source)) {
		t.Fatalf("derived digest/size = %q/%d", version.ContentSHA256, version.ByteSize)
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
INSERT INTO user_profiles (user_id, handle)
VALUES ('00000000-0000-4000-8000-000000000001', 'version-owner');
INSERT INTO categories (owner_user_id, name)
VALUES ('00000000-0000-4000-8000-000000000001', 'Version Owner');
`); err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	var categoryID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM categories WHERE name = 'Version Owner'`).Scan(&categoryID); err != nil {
		t.Fatalf("find category: %v", err)
	}

	library, err := postgres.NewLibraryRepository(db).Create(ctx, repository.CreateLibraryParams{
		OwnerUserID:  "00000000-0000-4000-8000-000000000001",
		CategoryName: "version owner",
		Name:         "UnionFind",
		Visibility:   domain.LibraryVisibilityPrivate,
	})
	if err == nil && library.CategoryName != "Version Owner" {
		t.Fatalf("category = %q, want the existing one found ignoring case", library.CategoryName)
	}
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
				LibraryID:  library.ID,
				SourceCode: fmt.Sprintf("# worker %d\nx = %d\n", i+1, i+1),
				CreatedBy:  userID,
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

func setURLParams(libraryID, actorUserID string, submissionURL *string) repository.SetLibraryVersionSubmissionURLParams {
	return repository.SetLibraryVersionSubmissionURLParams{
		LibraryID:     libraryID,
		VersionNumber: 1,
		ActorUserID:   actorUserID,
		SubmissionURL: submissionURL,
	}
}

// TestSetLibraryVersionSubmissionURLOwnerOnly covers the whole lifecycle: the
// owner sets, replaces, and clears the URL, reads return it, and a non-owner
// (even on a public library) or a missing version writes nothing.
func TestSetLibraryVersionSubmissionURLOwnerOnly(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, libraryID, _, ownerUserID := seedVersionFixture(t, ctx, db, "public")
	repo := postgres.NewLibraryVersionRepository(db)

	created, err := repo.FindByNumber(ctx, libraryID, 1, "")
	if err != nil {
		t.Fatalf("read new version: %v", err)
	}
	if created.SubmissionURL != "" || created.SubmissionURLUpdatedAt != nil {
		t.Fatalf("new version URL = %q / %v, want unset", created.SubmissionURL, created.SubmissionURLUpdatedAt)
	}

	first := "HTTPS://AtCoder.jp/contests/abc001/submissions/1"
	version, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, &first))
	if err != nil {
		t.Fatalf("owner set URL: %v", err)
	}
	if want := "https://AtCoder.jp/contests/abc001/submissions/1"; version.SubmissionURL != want {
		t.Fatalf("submission_url = %q, want normalized %q", version.SubmissionURL, want)
	}
	if version.SubmissionURLUpdatedAt == nil {
		t.Fatal("submission_url_updated_at is nil after set")
	}
	if version.SourceCode == "" || version.ContentSHA256 != created.ContentSHA256 {
		t.Fatalf("set must keep the immutable version body, got %+v", version)
	}

	second := "https://atcoder.jp/contests/abc001/submissions/2"
	if _, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, &second)); err != nil {
		t.Fatalf("owner replace URL: %v", err)
	}
	read, err := repo.FindByNumber(ctx, libraryID, 1, "")
	if err != nil || read.SubmissionURL != second {
		t.Fatalf("anonymous public read URL = %+v, %v; want %q", read, err, second)
	}
	summaries, err := repo.ListSummaries(ctx, repository.ListLibraryVersionsParams{LibraryID: libraryID, Limit: 20})
	if err != nil || len(summaries) != 1 || summaries[0].SubmissionURL != second {
		t.Fatalf("summaries = %+v, %v; want URL %q", summaries, err, second)
	}

	stranger := "00000000-0000-4000-8000-000000000002"
	other := "https://example.com/not-mine"
	_, err = repo.SetSubmissionURL(ctx, setURLParams(libraryID, stranger, &other))
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERSION_NOT_FOUND", "library version not found")
	_, err = repo.SetSubmissionURL(ctx, setURLParams(libraryID, stranger, nil))
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERSION_NOT_FOUND", "library version not found")
	missing := setURLParams(libraryID, ownerUserID, &other)
	missing.VersionNumber = 99
	_, err = repo.SetSubmissionURL(ctx, missing)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERSION_NOT_FOUND", "library version not found")
	if read, _ := repo.FindByNumber(ctx, libraryID, 1, ownerUserID); read.SubmissionURL != second {
		t.Fatalf("rejected writes changed URL to %q", read.SubmissionURL)
	}

	cleared, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, nil))
	if err != nil {
		t.Fatalf("owner clear URL: %v", err)
	}
	if cleared.SubmissionURL != "" || cleared.SubmissionURLUpdatedAt != nil {
		t.Fatalf("cleared URL = %q / %v, want unset", cleared.SubmissionURL, cleared.SubmissionURLUpdatedAt)
	}
	assertNoExecutionRows(t, ctx, db)
}

// TestSetLibraryVersionSubmissionURLValidatesURL proves the repository is the
// authoritative gate: invalid URLs are rejected before any write, and valid
// ones (Unicode hosts, IPv6, userinfo, max length) are stored trimmed.
func TestSetLibraryVersionSubmissionURLValidatesURL(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, libraryID, _, ownerUserID := seedVersionFixture(t, ctx, db, "private")
	repo := postgres.NewLibraryVersionRepository(db)

	for _, tc := range invalidSubmissionURLs {
		url := tc.url
		_, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, &url))
		assertAppError(t, err, apperror.ErrInvalidInput, "INVALID_SUBMISSION_URL", "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters")
	}
	if read, _ := repo.FindByNumber(ctx, libraryID, 1, ownerUserID); read.SubmissionURL != "" {
		t.Fatalf("invalid URLs stored %q", read.SubmissionURL)
	}
	for _, tc := range validSubmissionURLs {
		url := tc.url
		version, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, &url))
		if err != nil {
			t.Fatalf("valid URL %q (%s) rejected: %v", tc.url, tc.name, err)
		}
		if version.SubmissionURL != tc.url {
			t.Fatalf("stored URL = %q, want %q", version.SubmissionURL, tc.url)
		}
	}
	padded := "  https://example.com/trimmed  "
	version, err := repo.SetSubmissionURL(ctx, setURLParams(libraryID, ownerUserID, &padded))
	if err != nil || version.SubmissionURL != "https://example.com/trimmed" {
		t.Fatalf("padded URL stored as %+v, %v", version, err)
	}
}

// TestLibraryVersionSubmissionURLConstraints exercises the DB CHECK backstop
// and the narrowed immutability trigger with raw SQL.
func TestLibraryVersionSubmissionURLConstraints(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, _, versionID, _ := seedVersionFixture(t, ctx, db, "public")

	set := func(url string) error {
		_, err := db.Exec(ctx, `UPDATE library_versions SET submission_url = $2, submission_url_updated_at = now() WHERE id = $1::uuid`, versionID, url)
		return err
	}
	for _, tc := range invalidSubmissionURLs {
		assertCheckViolation(t, set(tc.url))
	}
	for _, tc := range validSubmissionURLs {
		if err := set(tc.url); err != nil {
			t.Fatalf("valid URL %q (%s) rejected by DB CHECK: %v", tc.url, tc.name, err)
		}
	}

	// URL and timestamp must be set or cleared together.
	_, err := db.Exec(ctx, `UPDATE library_versions SET submission_url_updated_at = NULL WHERE id = $1::uuid`, versionID)
	assertCheckViolation(t, err)

	// The trigger still rejects changes to the version body in the same UPDATE.
	_, err = db.Exec(ctx, `UPDATE library_versions SET submission_url = 'https://example.com/x', source_code = source_code || '# mutated' WHERE id = $1::uuid`, versionID)
	if err == nil || !strings.Contains(err.Error(), "library_versions are immutable") {
		t.Fatalf("body change alongside URL must hit the immutability trigger, got: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE library_versions SET submission_url = NULL, submission_url_updated_at = NULL WHERE id = $1::uuid`, versionID); err != nil {
		t.Fatalf("clearing the URL must be allowed: %v", err)
	}
}

func TestLibraryVersionReadsEnforceVisibilityInRepository(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, libraryID, _, ownerUserID := seedVersionFixture(t, ctx, db, "public")
	repo := postgres.NewLibraryVersionRepository(db)
	if _, err := repo.FindByNumber(ctx, libraryID, 1, ""); err != nil {
		t.Fatalf("anonymous public read: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}
	_, err := repo.FindByNumber(ctx, libraryID, 1, "")
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_VERSION_NOT_FOUND", "library version not found")
	if _, err := repo.FindByNumber(ctx, libraryID, 1, ownerUserID); err != nil {
		t.Fatalf("owner private read: %v", err)
	}
	versions, err := repo.ListSummaries(ctx, repository.ListLibraryVersionsParams{LibraryID: libraryID, Limit: 20})
	if err != nil || len(versions) != 0 {
		t.Fatalf("anonymous private versions = %+v, %v; want none", versions, err)
	}
	versions, err = repo.ListSummaries(ctx, repository.ListLibraryVersionsParams{LibraryID: libraryID, Limit: 20, ActorUserID: ownerUserID})
	if err != nil || len(versions) != 1 {
		t.Fatalf("owner private versions = %+v, %v; want one", versions, err)
	}
}

// TestLibraryUpdatedAtFollowsVersions pins "last updated": a new version bumps
// the library's updated_at, a submission URL change does not.
func TestLibraryUpdatedAtFollowsVersions(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	_, libraryID, _, ownerUserID := seedVersionFixture(t, ctx, db, "public")
	var publicID string
	if err := db.QueryRow(ctx, `SELECT public_id FROM libraries WHERE id = $1`, libraryID).Scan(&publicID); err != nil {
		t.Fatalf("read public id: %v", err)
	}

	libraries := postgres.NewLibraryRepository(db)
	updatedAt := func() time.Time {
		t.Helper()
		library, err := libraries.FindByPublicID(ctx, publicID)
		if err != nil {
			t.Fatalf("FindByPublicID: %v", err)
		}
		if library.ID != libraryID {
			t.Fatalf("library id = %q, want %q", library.ID, libraryID)
		}
		return library.UpdatedAt
	}

	before := updatedAt()
	versions := postgres.NewLibraryVersionRepository(db)
	url := "https://judge.example.com/submissions/1"
	if _, err := versions.SetSubmissionURL(ctx, repository.SetLibraryVersionSubmissionURLParams{
		LibraryID: libraryID, VersionNumber: 1, ActorUserID: ownerUserID, SubmissionURL: &url,
	}); err != nil {
		t.Fatalf("SetSubmissionURL: %v", err)
	}
	if got := updatedAt(); !got.Equal(before) {
		t.Fatalf("a submission URL change moved updated_at from %v to %v", before, got)
	}
	if _, err := versions.Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID: libraryID, SourceCode: "def f():\n    return 2\n", CreatedBy: ownerUserID,
	}); err != nil {
		t.Fatalf("Create version: %v", err)
	}
	if got := updatedAt(); !got.After(before) {
		t.Fatalf("a new version left updated_at at %v (was %v)", got, before)
	}
}
