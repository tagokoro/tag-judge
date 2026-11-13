package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// TestSiteWideListsPostgres pins the visibility rules of the three site-wide
// lists: only public libraries, only submissions to public problems (with the
// submitter's handle, or none), and every problem of the creator.
func TestSiteWideListsPostgres(t *testing.T) {
	databaseURL := os.Getenv("SUBMISSION_DETAILS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set SUBMISSION_DETAILS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db := submissionDetailsTestDatabase(t, ctx, databaseURL)

	const (
		alice = "00000000-0000-4000-8000-0000000000a1"
		bob   = "00000000-0000-4000-8000-0000000000b2"
	)
	if _, err := db.Exec(ctx, `
INSERT INTO users (id, provider, provider_user_id, name, email)
VALUES ('`+alice+`', 'test', 'alice', 'Alice', 'alice@example.test'),
       ('`+bob+`', 'test', 'bob', 'Bob', 'bob@example.test');
INSERT INTO user_profiles (user_id, handle) VALUES ('`+alice+`', 'alice');
INSERT INTO categories (owner_user_id, name) VALUES ('`+alice+`', 'Algorithms');
INSERT INTO libraries (category_id, name, visibility)
SELECT id, 'Public', 'public' FROM categories WHERE name = 'Algorithms';
INSERT INTO libraries (category_id, name, visibility)
SELECT id, 'Private', 'private' FROM categories WHERE name = 'Algorithms';
INSERT INTO problems (id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by)
VALUES (1, 'Public problem', 's', 1000, 262144, 'public', '`+alice+`'),
       (2, 'Private problem', 's', 1000, 262144, 'private', '`+alice+`'),
       (3, 'Bob problem', 's', 1000, 262144, 'public', '`+bob+`');
SELECT setval('problems_id_seq', 3);
INSERT INTO submissions (id, user_id, problem_id, language, source_code_key, test_cases_key, status, score)
VALUES (10, '`+alice+`', 1, 'python', 'a.py', 'cases.json', 'ac', 3),
       (11, '`+bob+`', 1, 'python', 'b.py', 'cases.json', 'wa', 1),
       (12, '`+alice+`', 2, 'python', 'c.py', 'cases.json', 'ac', 3);
`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	libraries, err := postgres.NewLibraryRepository(db).ListAllPublic(ctx, 20, 0)
	if err != nil {
		t.Fatalf("ListAllPublic: %v", err)
	}
	if len(libraries) != 1 || libraries[0].Name != "Public" || libraries[0].OwnerHandle != "alice" || libraries[0].CategoryName != "Algorithms" {
		t.Fatalf("public libraries = %+v, want only alice's Public library", libraries)
	}

	submissions, err := postgres.NewSubmissionRepository(db).ListRecentPublic(ctx, repository.ListRecentSubmissionsParams{Limit: 20})
	if err != nil {
		t.Fatalf("ListRecentPublic: %v", err)
	}
	got := map[int64]domain.PublicSubmission{}
	for _, submission := range submissions {
		got[submission.ID] = submission
	}
	if len(got) != 2 {
		t.Fatalf("recent submissions = %+v, want ids 10 and 11 only", submissions)
	}
	if _, leaked := got[12]; leaked {
		t.Fatal("a submission to a private problem must not be listed")
	}
	if got[10].SubmitterHandle != "alice" || got[10].ProblemTitle != "Public problem" {
		t.Fatalf("submission 10 = %+v", got[10])
	}
	if got[11].SubmitterHandle != "" {
		t.Fatalf("bob has no handle, handle = %q", got[11].SubmitterHandle)
	}

	problems, err := postgres.NewProblemRepository(db).ListByCreator(ctx, repository.ListProblemsByCreatorParams{CreatedBy: alice, Limit: 20})
	if err != nil {
		t.Fatalf("ListByCreator: %v", err)
	}
	if len(problems) != 2 || problems[0].ID != 2 || problems[1].ID != 1 {
		t.Fatalf("alice's problems = %+v, want ids 2 then 1 (private included)", problems)
	}
	if problems[0].CreatedByHandle != "alice" {
		t.Fatalf("created by handle = %q, want alice", problems[0].CreatedByHandle)
	}
	bobProblem, err := postgres.NewProblemRepository(db).FindByID(ctx, 3)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if bobProblem.CreatedByHandle != "" {
		t.Fatalf("bob has no handle, created by handle = %q", bobProblem.CreatedByHandle)
	}

	_, err = postgres.NewProblemRepository(db).Create(ctx, repository.CreateProblemParams{
		Title: "No handle", Statement: "s", TimeLimitMS: 1000, MemoryLimitKB: 262144,
		Visibility: domain.ProblemVisibilityPrivate, CreatedBy: bob,
	})
	assertAppError(t, err, apperror.ErrConflict, "HANDLE_REQUIRED", "create a handle before creating a problem")
	created, err := postgres.NewProblemRepository(db).Create(ctx, repository.CreateProblemParams{
		Title: "With handle", Statement: "s", TimeLimitMS: 1000, MemoryLimitKB: 262144,
		Visibility: domain.ProblemVisibilityPrivate, CreatedBy: alice,
	})
	if err != nil || created.CreatedByHandle != "alice" {
		t.Fatalf("Create = %+v, %v", created, err)
	}
}
