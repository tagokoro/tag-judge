-- name: ListPublicProblems :many
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM problems p
LEFT JOIN user_profiles up ON up.user_id = p.created_by
WHERE p.visibility = 'public'
ORDER BY p.id DESC
LIMIT $1 OFFSET $2;

-- name: GetPublicProblem :one
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM problems p
LEFT JOIN user_profiles up ON up.user_id = p.created_by
WHERE p.id = $1 AND p.visibility = 'public';

-- name: GetProblem :one
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM problems p
LEFT JOIN user_profiles up ON up.user_id = p.created_by
WHERE p.id = $1;

-- Insert and update return the creator's handle like the reads do. Only a
-- user with a handle may create a problem: no row comes back otherwise.
-- name: CreateProblem :one
WITH p AS (
    INSERT INTO problems (title, statement, time_limit_ms, memory_limit_kb, visibility, created_by)
    SELECT sqlc.arg(title), sqlc.arg(statement), sqlc.arg(time_limit_ms), sqlc.arg(memory_limit_kb), sqlc.arg(visibility), sqlc.arg(created_by)::uuid
    WHERE EXISTS (SELECT 1 FROM user_profiles WHERE user_id = sqlc.arg(created_by)::uuid)
    RETURNING *
)
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM p
LEFT JOIN user_profiles up ON up.user_id = p.created_by;

-- name: UpdateProblem :one
WITH p AS (
    UPDATE problems
    SET title = sqlc.arg(title),
        statement = sqlc.arg(statement),
        time_limit_ms = sqlc.arg(time_limit_ms),
        memory_limit_kb = sqlc.arg(memory_limit_kb),
        visibility = sqlc.arg(visibility),
        updated_at = now()
    WHERE id = sqlc.arg(id)
    RETURNING *
)
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM p
LEFT JOIN user_profiles up ON up.user_id = p.created_by;

-- name: SetCurrentTestCasesKey :exec
UPDATE problems
SET current_test_cases_key = $2,
    updated_at = now()
WHERE id = $1;

-- name: DeleteProblem :execrows
DELETE FROM problems
WHERE id = $1;

-- Every problem the user created, including private ones, newest first.
-- name: ListProblemsByCreator :many
SELECT p.id, p.title, p.statement, p.time_limit_ms, p.memory_limit_kb, p.visibility, p.current_test_cases_key, p.created_by::text AS created_by, up.handle AS created_by_handle, p.created_at, p.updated_at
FROM problems p
LEFT JOIN user_profiles up ON up.user_id = p.created_by
WHERE p.created_by = ((sqlc.arg(created_by)::text)::uuid)
ORDER BY p.id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
