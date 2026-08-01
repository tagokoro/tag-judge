-- name: CreateSubmission :one
INSERT INTO submissions (
    user_id,
    problem_id,
    language,
    source_code_key,
    status,
    score
)
VALUES (
    ((sqlc.arg(user_id)::text)::uuid),
    sqlc.arg(problem_id),
    sqlc.arg(language),
    sqlc.arg(source_code_key),
    sqlc.arg(status),
    sqlc.arg(score)
)
RETURNING id, user_id::text AS user_id, problem_id, language, source_code_key, status, score, created_at, judged_at;

-- name: GetSubmission :one
SELECT id, user_id::text AS user_id, problem_id, language, source_code_key, status, score, created_at, judged_at
FROM submissions
WHERE id = $1;

-- name: ListUserSubmissions :many
SELECT id, user_id::text AS user_id, problem_id, language, source_code_key, status, score, created_at, judged_at
FROM submissions
WHERE user_id = ((sqlc.arg(user_id)::text)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
