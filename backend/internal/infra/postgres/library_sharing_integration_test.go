package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
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
INSERT INTO problems (id, title, statement, time_limit_ms, memory_limit_kb, created_by)
VALUES (42, 'Test', 'Test', 1000, 262144, '00000000-0000-4000-8000-000000000001');
INSERT INTO submissions (id, user_id, problem_id, language, source_code_key, test_cases_key, status, score)
VALUES (1234, '00000000-0000-4000-8000-000000000001', 42, 'python', 'source.py', 'cases.json', 'pending', 0);
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
	if _, err := db.Exec(ctx, `
INSERT INTO library_verifications (library_version_id, submission_id, adapter_source, adapter_sha256)
VALUES ((SELECT id FROM library_versions WHERE version_number = 1), 1234,
        $1,
        encode(digest(convert_to($1, 'UTF8'), 'sha256'), 'hex'));
`, "print(add(1, 2))\n"); err != nil {
		t.Fatalf("seed library verification: %v", err)
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

	// adapter_sha256 must equal the hash of the stored adapter bytes.
	if _, err := db.Exec(ctx, `
INSERT INTO library_verifications (library_version_id, submission_id, adapter_source, adapter_sha256)
VALUES ((SELECT id FROM library_versions WHERE version_number = 1), 1234,
        'print("wrong adapter")' || chr(10),
        encode(digest(convert_to('print(add(1, 2))' || chr(10), 'UTF8'), 'sha256'), 'hex'))`); err == nil {
		t.Fatal("adapter_sha256 mismatch must be rejected by the DB constraint")
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
				ContentSHA256: librarycombine.ContentSHA256(fmt.Sprintf("# worker %d\nx = %d\n", i+1, i+1)),
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

// seedVerificationFixture creates one user, namespace, public library, version,
// and problem with test cases, and returns the repository identifiers needed
// to start a verification.
func seedVerificationFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool, visibility string) (namespaceID, libraryID, versionID, actorUserID string) {
	t.Helper()
	actorUserID = "00000000-0000-4000-8000-000000000001"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'verify-owner', 'Owner', 'verify-owner@example.test');`, actorUserID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO namespaces (owner_user_id, slug, display_name)
VALUES ($1, 'verify-author', 'Verify Author');`, actorUserID); err != nil {
		t.Fatalf("seed namespace: %v", err)
	}
	if _, err := db.Exec(ctx, `
INSERT INTO problems (id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by)
VALUES (42, 'A+B', 'add two numbers', 2000, 262144, 'public', 'problems/42/testcases/abc.json', $1);`, actorUserID); err != nil {
		t.Fatalf("seed problem: %v", err)
	}
	if err := db.QueryRow(ctx, `SELECT id::text FROM namespaces WHERE slug = 'verify-author'`).Scan(&namespaceID); err != nil {
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
	source := "def add(a, b):\n    return a + b\n\nif __name__ == \"__main__\":\n    print(\"library-main\")\n"
	version, err := postgres.NewLibraryVersionRepository(db).Create(ctx, repository.CreateLibraryVersionParams{
		LibraryID:     library.ID,
		SourceCode:    source,
		ContentSHA256: librarycombine.ContentSHA256(source),
		ByteSize:      len([]byte(source)),
		CreatedBy:     actorUserID,
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	return namespaceID, library.ID, version.ID, actorUserID
}

// TestLibraryVerificationOutboxPayloadMatchesSubmissionJob proves that verify
// feeds the existing Submission/Outbox path: the payload is byte-shape
// identical to a regular submission's job message.
func TestLibraryVerificationOutboxPayloadMatchesSubmissionJob(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, actorUserID := seedVerificationFixture(t, ctx, db, "public")

	adapter := "a, b = map(int, input().split())\nprint(add(a, b))\n"
	verifications := postgres.NewLibraryVerificationRepository(db)
	verification, submission, err := verifications.CreateWithSubmission(ctx, repository.CreateLibraryVerificationParams{
		ActorUserID:      actorUserID,
		NamespaceID:      namespaceID,
		LibraryID:        libraryID,
		LibraryVersionID: versionID,
		ProblemID:        42,
		Language:         domain.SubmissionLanguagePython,
		SourceCodeKey:    "submissions/" + actorUserID + "/problems/42/test1234.py",
		TestCasesKey:     "problems/42/testcases/abc.json",
		TimeLimitMS:      2000,
		AdapterSource:    adapter,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if submission.Status != domain.SubmissionStatusPending || submission.Language != domain.SubmissionLanguagePython || submission.ProblemID != 42 {
		t.Fatalf("submission = %+v", submission)
	}
	if verification.SubmissionID != submission.ID || verification.Status != domain.SubmissionStatusPending || verification.Score != 0 {
		t.Fatalf("verification = %+v", verification)
	}
	if verification.AdapterSHA256 != librarycombine.ContentSHA256(adapter) {
		t.Fatalf("adapter sha = %q", verification.AdapterSHA256)
	}

	// The submissions row and the outbox payload must match the regular path.
	var status string
	var testCasesKey string
	if err := db.QueryRow(ctx, `SELECT status, test_cases_key FROM submissions WHERE id = $1`, submission.ID).Scan(&status, &testCasesKey); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if status != "pending" || testCasesKey != "problems/42/testcases/abc.json" {
		t.Fatalf("submission row status=%q testCasesKey=%q", status, testCasesKey)
	}

	var payloadRaw []byte
	if err := db.QueryRow(ctx, `SELECT payload FROM submission_job_outbox WHERE submission_id = $1`, submission.ID).Scan(&payloadRaw); err != nil {
		t.Fatalf("read outbox payload: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	wantKeys := map[string]bool{"submissionId": true, "language": true, "sourceCodeKey": true, "testCasesKey": true, "timeLimitMs": true}
	if len(payload) != len(wantKeys) {
		t.Fatalf("payload keys = %v, want exactly %v", payload, wantKeys)
	}
	for key := range payload {
		if !wantKeys[key] {
			t.Fatalf("unexpected payload key %q", key)
		}
	}
	var payloadSubmissionID, language, payloadSourceCodeKey, payloadTestCasesKey string
	var payloadTimeLimitMS int
	if err := json.Unmarshal(payload["submissionId"], &payloadSubmissionID); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["language"], &language); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["sourceCodeKey"], &payloadSourceCodeKey); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["testCasesKey"], &payloadTestCasesKey); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(payload["timeLimitMs"], &payloadTimeLimitMS); err != nil {
		t.Fatal(err)
	}
	if payloadSubmissionID != fmt.Sprintf("%d", submission.ID) || language != "python" || payloadSourceCodeKey != "submissions/"+actorUserID+"/problems/42/test1234.py" || payloadTestCasesKey != "problems/42/testcases/abc.json" || payloadTimeLimitMS != 2000 {
		t.Fatalf("payload = %+v", payload)
	}

	// library_verifications must not duplicate problem/owner columns.
	var submissionID int64
	var adapterSha string
	if err := db.QueryRow(ctx, `SELECT submission_id, adapter_sha256 FROM library_verifications WHERE id = $1`, verification.ID).Scan(&submissionID, &adapterSha); err != nil {
		t.Fatalf("read verification: %v", err)
	}
	if submissionID != submission.ID || adapterSha != librarycombine.ContentSHA256(adapter) {
		t.Fatalf("verification row submission=%d sha=%q", submissionID, adapterSha)
	}
}

// TestLibraryVerificationRollsBackSubmissionAndOutbox proves that a failed
// re-check (hidden library, foreign version) commits nothing.
func TestLibraryVerificationRollsBackSubmissionAndOutbox(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, actorUserID := seedVerificationFixture(t, ctx, db, "public")

	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}

	verifications := postgres.NewLibraryVerificationRepository(db)
	_, _, err := verifications.CreateWithSubmission(ctx, repository.CreateLibraryVerificationParams{
		ActorUserID:      "00000000-0000-4000-8000-000000000002",
		NamespaceID:      namespaceID,
		LibraryID:        libraryID,
		LibraryVersionID: versionID,
		ProblemID:        42,
		Language:         domain.SubmissionLanguagePython,
		SourceCodeKey:    "submissions/x/problems/42/a.py",
		TestCasesKey:     "problems/42/testcases/abc.json",
		TimeLimitMS:      2000,
		AdapterSource:    "print(add(1, 2))\n",
	})
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")

	countSubmissions := func() int64 {
		var count int64
		if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM submissions WHERE problem_id = 42`).Scan(&count); err != nil {
			t.Fatalf("count submissions: %v", err)
		}
		return count
	}
	var outbox int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM submission_job_outbox`).Scan(&outbox); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	var verRow int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM library_verifications`).Scan(&verRow); err != nil {
		t.Fatalf("count verifications: %v", err)
	}
	if countSubmissions() != 0 || outbox != 0 || verRow != 0 {
		t.Fatalf("rollback leaked rows: submissions=%d outbox=%d verifications=%d", countSubmissions(), outbox, verRow)
	}

	// A version id outside of the library fails the same way.
	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'public' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("restore visibility: %v", err)
	}
	_, _, err = verifications.CreateWithSubmission(ctx, repository.CreateLibraryVerificationParams{
		ActorUserID:      actorUserID,
		NamespaceID:      namespaceID,
		LibraryID:        libraryID,
		LibraryVersionID: "11111111-1111-4111-8111-111111111111", // not this library's version
		ProblemID:        42,
		Language:         domain.SubmissionLanguagePython,
		SourceCodeKey:    "submissions/x/problems/42/b.py",
		TestCasesKey:     "problems/42/testcases/abc.json",
		TimeLimitMS:      2000,
		AdapterSource:    "print(add(1, 2))\n",
	})
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	if countSubmissions() != 0 || outbox != 0 || verRow != 0 {
		t.Fatalf("bad version leaked rows: submissions=%d outbox=%d verifications=%d", countSubmissions(), outbox, verRow)
	}
}

// TestLibraryVerificationRechecksVisibilityInTransaction proves the FOR SHARE
// re-check: an owner-only private library is rejected inside the transaction
// even when the pre-check would pass.
func TestLibraryVerificationRechecksVisibilityInTransaction(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, ownerUserID := seedVerificationFixture(t, ctx, db, "public")

	stranger := "00000000-0000-4000-8000-000000000003"
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ($1, 'test', 'verify-stranger', 'Stranger', 'stranger@example.test');`, stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}

	verifications := postgres.NewLibraryVerificationRepository(db)
	params := repository.CreateLibraryVerificationParams{
		ActorUserID:      stranger,
		NamespaceID:      namespaceID,
		LibraryID:        libraryID,
		LibraryVersionID: versionID,
		ProblemID:        42,
		Language:         domain.SubmissionLanguagePython,
		SourceCodeKey:    "submissions/" + stranger + "/problems/42/c.py",
		TestCasesKey:     "problems/42/testcases/abc.json",
		TimeLimitMS:      2000,
		AdapterSource:    "print(add(1, 2))\n",
	}

	// Public: a stranger may verify.
	if _, _, err := verifications.CreateWithSubmission(ctx, params); err != nil {
		t.Fatalf("public verify: %v", err)
	}

	// Owner flips private; the same verify now commits nothing.
	if _, err := db.Exec(ctx, `UPDATE libraries SET visibility = 'private' WHERE id = $1::uuid`, libraryID); err != nil {
		t.Fatalf("flip visibility: %v", err)
	}
	before := int64(0)
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM submissions WHERE problem_id = 42`).Scan(&before); err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	_, _, err := verifications.CreateWithSubmission(ctx, params)
	assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")

	after := int64(0)
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM submissions WHERE problem_id = 42`).Scan(&after); err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if before != 1 || after != 1 {
		t.Fatalf("private verify must commit nothing: before=%d after=%d", before, after)
	}

	// The owner may still verify their own private library.
	params.ActorUserID = ownerUserID
	params.SourceCodeKey = "submissions/" + ownerUserID + "/problems/42/d.py"
	if _, _, err := verifications.CreateWithSubmission(ctx, params); err != nil {
		t.Fatalf("owner private verify: %v", err)
	}
}

// TestLibraryVerificationVisibilityTOCTOU uses two independent connections and
// a barrier: the visibility UPDATE holds its row lock while verification starts,
// then commits. The verification must observe the committed private state rather
// than a stale pre-lock snapshot.
func TestLibraryVerificationVisibilityTOCTOU(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)
	namespaceID, libraryID, versionID, _ := seedVerificationFixture(t, ctx, db, "public")
	stranger := "00000000-0000-4000-8000-000000000003"
	if _, err := db.Exec(ctx, `INSERT INTO users (id, provider, provider_user_id, name, email) VALUES ($1, 'test', 'toctou-stranger', 'Stranger', 'toctou@example.test')`, stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}

	// Use a dedicated one-connection pool so pg_stat_activity can identify the
	// verification transaction by application_name. The observation barrier is
	// deliberately the database's Lock wait, not merely goroutine scheduling.
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
	baselineCounts := make(map[string]int64, 3)
	for _, table := range []string{"submissions", "submission_job_outbox", "library_verifications"} {
		var count int64
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count baseline %s: %v", table, err)
		}
		baselineCounts[table] = count
	}

	verificationStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(verificationStarted)
		_, _, verifyErr := postgres.NewLibraryVerificationRepository(workerDB).CreateWithSubmission(ctx, repository.CreateLibraryVerificationParams{
			ActorUserID: stranger, NamespaceID: namespaceID, LibraryID: libraryID, LibraryVersionID: versionID,
			ProblemID: 42, Language: domain.SubmissionLanguagePython,
			SourceCodeKey: "submissions/" + stranger + "/problems/42/toctou.py",
			TestCasesKey:  "problems/42/testcases/abc.json", TimeLimitMS: 2000, AdapterSource: "print(1)\n",
		})
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
				t.Fatalf("observed unexpected verification lock query: %q", query)
			}
			if waitEventType != "Lock" {
				t.Fatalf("verification wait_event_type = %q, want Lock", waitEventType)
			}
			observedLockWait = true
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("observe verification lock wait: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observedLockWait {
		_ = tx.Rollback(ctx)
		<-result
		t.Fatal("timed out waiting to observe LockLibraryForVerification FOR SHARE lock wait")
	}
	// Without FOR SHARE this ordering is not guaranteed: the verifier could
	// read public before the updater commits and successfully insert rows.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit visibility update: %v", err)
	}
	if err := <-result; err == nil {
		t.Fatal("verification succeeded despite concurrent visibility change")
	} else {
		assertAppError(t, err, apperror.ErrNotFound, "LIBRARY_NOT_FOUND", "library not found")
	}
	for table, want := range baselineCounts {
		var got int64
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d after rejected verification", table, got, want)
		}
	}
}
