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
