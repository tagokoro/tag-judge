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

// seedVersionFixture creates one user (the namespace owner), namespace, library,
// and version. A submission URL needs no Problem, Submission, or Outbox, so
// this fixture deliberately seeds none of them.
func seedVersionFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, visibility string) (namespaceID, libraryID, versionID, ownerUserID string) {
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
		LibraryID:  library.ID,
		SourceCode: source,
		CreatedBy:  ownerUserID,
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	return namespaceID, library.ID, version.ID, ownerUserID
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

func TestSubmissionURLLivesOnLibraryVersions(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	var links, verifications bool
	if err := db.QueryRow(ctx, `SELECT
    EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = 'library_submission_links'),
    EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = current_schema() AND tablename = 'library_verifications')`).Scan(&links, &verifications); err != nil {
		t.Fatalf("inspect tables: %v", err)
	}
	if links || verifications {
		t.Fatalf("separate link tables must not exist: library_submission_links = %v, library_verifications = %v", links, verifications)
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
