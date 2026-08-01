-- name: ListPublicProblems :many
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text AS created_by, created_at, updated_at
FROM problems
WHERE visibility = 'public'
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: GetPublicProblem :one
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text AS created_by, created_at, updated_at
FROM problems
WHERE id = $1 AND visibility = 'public';

-- name: CreateProblem :one
INSERT INTO problems (title, statement, time_limit_ms, memory_limit_kb, visibility, created_by)
VALUES ($1, $2, $3, $4, $5, sqlc.arg(created_by)::uuid)
RETURNING id, title, statement, time_limit_ms, memory_limit_kb, visibility, created_by::text AS created_by, created_at, updated_at;

-- name: CreateTestCase :exec
INSERT INTO test_cases (problem_id, input, expected_output, is_sample, score)
VALUES ($1, $2, $3, $4, $5);
