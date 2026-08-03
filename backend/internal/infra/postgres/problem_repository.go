package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type ProblemRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewProblemRepository(db *pgxpool.Pool) *ProblemRepository {
	return &ProblemRepository{db: db, queries: sqlc.New(db)}
}

func (r *ProblemRepository) ListPublic(ctx context.Context, params repository.ListProblemsParams) ([]domain.Problem, error) {
	rows, err := r.queries.ListPublicProblems(ctx, sqlc.ListPublicProblemsParams{
		Limit:  int32(params.Limit),
		Offset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}

	problems := make([]domain.Problem, 0, len(rows))
	for _, row := range rows {
		problems = append(problems, newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt))
	}

	return problems, nil
}

func (r *ProblemRepository) ListVisible(ctx context.Context, params repository.ListProblemsParams, actorUserID string, canSeeAll bool) ([]domain.Problem, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text, created_at, updated_at
FROM problems
WHERE visibility = 'public'
   OR $3
   OR ($4 <> '' AND created_by = $4::uuid)
ORDER BY id DESC
LIMIT $1 OFFSET $2`, params.Limit, params.Offset, canSeeAll, actorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	problems := []domain.Problem{}
	for rows.Next() {
		problem, err := scanProblem(rows)
		if err != nil {
			return nil, err
		}
		problems = append(problems, *problem)
	}
	return problems, rows.Err()
}

func (r *ProblemRepository) FindPublicByID(ctx context.Context, id int64) (*domain.Problem, error) {
	row, err := r.queries.GetPublicProblem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}

	problem := newProblem(row.ID, row.Title, row.Statement, row.TimeLimitMs, row.MemoryLimitKb, row.Visibility, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &problem, nil
}

func (r *ProblemRepository) FindVisibleByID(ctx context.Context, id int64, actorUserID string, canSeeAll bool) (*domain.Problem, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text, created_at, updated_at
FROM problems
WHERE id = $1
  AND (
    visibility = 'public'
    OR $2
    OR ($3 <> '' AND created_by = $3::uuid)
  )`, id, canSeeAll, actorUserID)

	problem, err := scanProblem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}
	return problem, nil
}

func (r *ProblemRepository) Create(ctx context.Context, params repository.ProblemMutationParams) (*domain.Problem, error) {
	row := r.db.QueryRow(ctx, `
INSERT INTO problems (title, statement, time_limit_ms, memory_limit_kb, visibility, created_by)
VALUES ($1, $2, $3, $4, $5, $6::uuid)
RETURNING id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text, created_at, updated_at`,
		params.Title,
		params.Statement,
		params.TimeLimitMS,
		params.MemoryLimitKB,
		string(params.Visibility),
		params.CreatedBy,
	)
	return scanProblem(row)
}

func (r *ProblemRepository) Update(ctx context.Context, id int64, params repository.ProblemMutationParams) (*domain.Problem, error) {
	row := r.db.QueryRow(ctx, `
UPDATE problems
SET title = $2,
    statement = $3,
    time_limit_ms = $4,
    memory_limit_kb = $5,
    visibility = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text, created_at, updated_at`,
		id,
		params.Title,
		params.Statement,
		params.TimeLimitMS,
		params.MemoryLimitKB,
		string(params.Visibility),
	)
	problem, err := scanProblem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	if err != nil {
		return nil, err
	}
	return problem, nil
}

func (r *ProblemRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM problems WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
	}
	return nil
}

func (r *ProblemRepository) ListTestCases(ctx context.Context, problemID int64) ([]domain.TestCase, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, problem_id, input, expected_output, is_sample, score, created_at
FROM test_cases
WHERE problem_id = $1
ORDER BY id`, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	testCases := []domain.TestCase{}
	for rows.Next() {
		testCase, err := scanTestCase(rows)
		if err != nil {
			return nil, err
		}
		testCases = append(testCases, *testCase)
	}
	return testCases, rows.Err()
}

func (r *ProblemRepository) CreateTestCase(ctx context.Context, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	row := r.db.QueryRow(ctx, `
INSERT INTO test_cases (problem_id, input, expected_output, is_sample, score)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, problem_id, input, expected_output, is_sample, score, created_at`,
		params.ProblemID,
		params.Input,
		params.ExpectedOutput,
		params.IsSample,
		params.Score,
	)
	return scanTestCase(row)
}

func (r *ProblemRepository) UpdateTestCase(ctx context.Context, id int64, params repository.TestCaseMutationParams) (*domain.TestCase, error) {
	row := r.db.QueryRow(ctx, `
UPDATE test_cases
SET input = $2,
    expected_output = $3,
    is_sample = $4,
    score = $5
WHERE id = $1 AND problem_id = $6
RETURNING id, problem_id, input, expected_output, is_sample, score, created_at`,
		id,
		params.Input,
		params.ExpectedOutput,
		params.IsSample,
		params.Score,
		params.ProblemID,
	)
	testCase, err := scanTestCase(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("TEST_CASE_NOT_FOUND", "test case not found")
	}
	if err != nil {
		return nil, err
	}
	return testCase, nil
}

func (r *ProblemRepository) DeleteTestCase(ctx context.Context, problemID int64, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM test_cases WHERE id = $1 AND problem_id = $2`, id, problemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("TEST_CASE_NOT_FOUND", "test case not found")
	}
	return nil
}

func (r *ProblemRepository) CreateSubmission(ctx context.Context, params repository.CreateSubmissionParams) (*domain.Submission, error) {
	row := r.db.QueryRow(ctx, `
INSERT INTO submissions (user_id, problem_id, language, source_code)
VALUES ($1::uuid, $2, $3, $4)
RETURNING id, user_id::text, problem_id, language, source_code, status, score, created_at, judged_at`,
		params.UserID,
		params.ProblemID,
		params.Language,
		params.SourceCode,
	)
	return scanSubmission(row)
}

func (r *ProblemRepository) ListSubmissionsByProblem(ctx context.Context, problemID int64, userID string, canSeeAll bool) ([]domain.Submission, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, user_id::text, problem_id, language, source_code, status, score, created_at, judged_at
FROM submissions
WHERE problem_id = $1 AND ($2 OR user_id = $3::uuid)
ORDER BY created_at DESC, id DESC
LIMIT 50`, problemID, canSeeAll, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	submissions := []domain.Submission{}
	for rows.Next() {
		submission, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, *submission)
	}
	return submissions, rows.Err()
}

func (r *ProblemRepository) FindSubmissionByID(ctx context.Context, id int64) (*domain.Submission, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, user_id::text, problem_id, language, source_code, status, score, created_at, judged_at
FROM submissions
WHERE id = $1`, id)
	submission, err := scanSubmission(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("SUBMISSION_NOT_FOUND", "submission not found")
	}
	if err != nil {
		return nil, err
	}

	results, err := r.listSubmissionResults(ctx, id)
	if err != nil {
		return nil, err
	}
	submission.Results = results
	return submission, nil
}

func (r *ProblemRepository) listSubmissionResults(ctx context.Context, submissionID int64) ([]domain.SubmissionResult, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, submission_id, test_case_id, status, execution_time_ms, memory_kb, stdout, stderr, created_at
FROM submission_results
WHERE submission_id = $1
ORDER BY id`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []domain.SubmissionResult{}
	for rows.Next() {
		var result domain.SubmissionResult
		var executionTimeMS pgtype.Int4
		var memoryKB pgtype.Int4
		if err := rows.Scan(
			&result.ID,
			&result.SubmissionID,
			&result.TestCaseID,
			&result.Status,
			&executionTimeMS,
			&memoryKB,
			&result.Stdout,
			&result.Stderr,
			&result.CreatedAt,
		); err != nil {
			return nil, err
		}
		if executionTimeMS.Valid {
			value := int(executionTimeMS.Int32)
			result.ExecutionTimeMS = &value
		}
		if memoryKB.Valid {
			value := int(memoryKB.Int32)
			result.MemoryKB = &value
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProblem(row scanner) (*domain.Problem, error) {
	var problem domain.Problem
	if err := row.Scan(
		&problem.ID,
		&problem.Title,
		&problem.Statement,
		&problem.TimeLimitMS,
		&problem.MemoryLimitKB,
		&problem.Visibility,
		&problem.CreatedBy,
		&problem.CreatedAt,
		&problem.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &problem, nil
}

func scanTestCase(row scanner) (*domain.TestCase, error) {
	var testCase domain.TestCase
	if err := row.Scan(
		&testCase.ID,
		&testCase.ProblemID,
		&testCase.Input,
		&testCase.ExpectedOutput,
		&testCase.IsSample,
		&testCase.Score,
		&testCase.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &testCase, nil
}

func scanSubmission(row scanner) (*domain.Submission, error) {
	var submission domain.Submission
	var judgedAt pgtype.Timestamptz
	if err := row.Scan(
		&submission.ID,
		&submission.UserID,
		&submission.ProblemID,
		&submission.Language,
		&submission.SourceCode,
		&submission.Status,
		&submission.Score,
		&submission.CreatedAt,
		&judgedAt,
	); err != nil {
		return nil, err
	}
	if judgedAt.Valid {
		submission.JudgedAt = &judgedAt.Time
	}
	return &submission, nil
}

func newProblem(
	id int64,
	title string,
	statement string,
	timeLimitMS int32,
	memoryLimitKB int32,
	visibility string,
	createdBy string,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) domain.Problem {
	return domain.Problem{
		ID:            id,
		Title:         title,
		Statement:     statement,
		TimeLimitMS:   int(timeLimitMS),
		MemoryLimitKB: int(memoryLimitKB),
		Visibility:    domain.ProblemVisibility(visibility),
		CreatedBy:     createdBy,
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}
}
