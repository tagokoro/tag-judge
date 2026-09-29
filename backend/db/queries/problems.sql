-- name: ListPublicProblems :many
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by::text AS created_by, created_at, updated_at
FROM problems
WHERE visibility = 'public'
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: GetPublicProblem :one
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by::text AS created_by, created_at, updated_at
FROM problems
WHERE id = $1 AND visibility = 'public';

-- name: GetProblem :one
SELECT id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by::text AS created_by, created_at, updated_at
FROM problems
WHERE id = $1;

-- name: CreateProblem :one
INSERT INTO problems (title, statement, time_limit_ms, memory_limit_kb, visibility, created_by)
VALUES ($1, $2, $3, $4, $5, sqlc.arg(created_by)::uuid)
RETURNING id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by::text AS created_by, created_at, updated_at;

-- name: UpdateProblem :one
UPDATE problems
SET title = sqlc.arg(title),
    statement = sqlc.arg(statement),
    time_limit_ms = sqlc.arg(time_limit_ms),
    memory_limit_kb = sqlc.arg(memory_limit_kb),
    visibility = sqlc.arg(visibility),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, title, statement, time_limit_ms, memory_limit_kb, visibility, current_test_cases_key, created_by::text AS created_by, created_at, updated_at;

-- name: SetCurrentTestCasesKey :exec
UPDATE problems
SET current_test_cases_key = $2,
    updated_at = now()
WHERE id = $1;

-- name: DeleteProblem :execrows
DELETE FROM problems
WHERE id = $1;
