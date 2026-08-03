-- name: CreateCodeRun :one
INSERT INTO code_runs (user_id, language, source_code, stdin, status)
VALUES ($1, $2, $3, $4, 'pending')
RETURNING id, user_id::text AS user_id, language, source_code, stdin, stdout, stderr, status,
          execution_time_ms, memory_kb, created_at;

-- name: GetCodeRun :one
SELECT id, user_id::text AS user_id, language, source_code, stdin, stdout, stderr, status,
       execution_time_ms, memory_kb, created_at
FROM code_runs
WHERE id = $1;

-- name: ListCodeRunsByUserID :many
SELECT id, user_id::text AS user_id, language, source_code, stdin, stdout, stderr, status,
       execution_time_ms, memory_kb, created_at
FROM code_runs
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;
