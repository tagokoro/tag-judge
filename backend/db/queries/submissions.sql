-- name: CreateSubmission :one
INSERT INTO submissions (
    user_id,
    problem_id,
    language,
    source_code_key,
	 test_cases_key,
    status,
    score,
    is_public
)
VALUES (
    ((sqlc.arg(user_id)::text)::uuid),
    sqlc.arg(problem_id),
    sqlc.arg(language),
    sqlc.arg(source_code_key),
	 sqlc.arg(test_cases_key),
    sqlc.arg(status),
    sqlc.arg(score),
    sqlc.arg(is_public)
)
RETURNING id, user_id::text AS user_id, problem_id, language, source_code_key, test_cases_key, status, score, is_public, created_at, judged_at;

-- Reads name the problem and the submitter (by handle, when they have one).
-- name: GetSubmission :one
SELECT s.id, s.user_id::text AS user_id, s.problem_id, s.language, s.source_code_key, s.test_cases_key, s.status, s.score, s.is_public, s.created_at, s.judged_at,
       p.title AS problem_title, up.handle AS submitter_handle
FROM submissions s
JOIN problems p ON p.id = s.problem_id
LEFT JOIN user_profiles up ON up.user_id = s.user_id
WHERE s.id = $1;

-- A null problem_id lists the user's submissions to every problem.
-- name: ListUserSubmissions :many
SELECT s.id, s.user_id::text AS user_id, s.problem_id, s.language, s.source_code_key, s.test_cases_key, s.status, s.score, s.is_public, s.created_at, s.judged_at,
       p.title AS problem_title, up.handle AS submitter_handle
FROM submissions s
JOIN problems p ON p.id = s.problem_id
LEFT JOIN user_profiles up ON up.user_id = s.user_id
WHERE s.user_id = ((sqlc.arg(user_id)::text)::uuid)
  AND (sqlc.narg(problem_id)::bigint IS NULL OR s.problem_id = sqlc.narg(problem_id)::bigint)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: ListSubmissionCaseResults :many
SELECT test_case_id, status, execution_time_ms, memory_kb
FROM submission_results
WHERE submission_id = $1
ORDER BY test_case_id, id;

-- Recent public submissions for the site-wide submission list. A submission
-- made while its problem was private stays out even after the problem is
-- published. Only public metadata is returned: no source key and no user id.
-- The submitter is shown by their handle when they have one.
-- name: ListRecentPublicSubmissions :many
SELECT s.id, s.problem_id, p.title AS problem_title, s.language, s.status, s.score, s.created_at, s.judged_at,
       up.handle AS submitter_handle
FROM submissions s
JOIN problems p ON p.id = s.problem_id
LEFT JOIN user_profiles up ON up.user_id = s.user_id
WHERE s.is_public
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
