-- The transaction re-checks whom the library belongs to and whether the
-- version still belongs to the library using this single locked snapshot.
-- libraries is locked with FOR SHARE so a concurrent visibility PATCH waits.
-- name: LockLibraryForVerification :one
SELECT l.visibility, n.owner_user_id
FROM namespaces n
JOIN libraries l ON l.namespace_id = n.id
WHERE n.id = ((sqlc.arg(namespace_id)::text)::uuid)
  AND l.id = ((sqlc.arg(library_id)::text)::uuid)
FOR SHARE OF l;

-- name: CountLibraryVersionInLibrary :one
SELECT COUNT(*) AS count
FROM library_versions
WHERE id = ((sqlc.arg(library_version_id)::text)::uuid)
  AND library_id = ((sqlc.arg(library_id)::text)::uuid);

-- name: InsertLibraryVerification :one
INSERT INTO library_verifications (library_version_id, submission_id, adapter_source, adapter_sha256)
VALUES (((sqlc.arg(library_version_id)::text)::uuid), sqlc.arg(submission_id), sqlc.arg(adapter_source), sqlc.arg(adapter_sha256))
RETURNING id, created_at;

-- The verification detail reads problem, starter, and status from the joined
-- submissions row; those are not duplicated on library_verifications.
-- name: GetLibraryVerification :one
SELECT v.id, v.library_version_id::text AS library_version_id, v.submission_id,
       v.adapter_source, v.adapter_sha256, v.created_at,
       s.problem_id, s.user_id::text AS user_id, s.status, s.score, s.judged_at,
       lv.version_number, lv.library_id::text AS library_id,
       l.slug AS library_slug, n.slug AS namespace_slug
FROM library_verifications v
JOIN submissions s ON s.id = v.submission_id
JOIN library_versions lv ON lv.id = v.library_version_id
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE v.id = sqlc.arg(verification_id);

-- name: ListLibraryVerificationsByUser :many
SELECT v.id, v.library_version_id::text AS library_version_id, v.submission_id,
       v.adapter_sha256, v.created_at,
       s.problem_id, s.user_id::text AS user_id, s.status, s.score, s.judged_at,
       lv.version_number, lv.library_id::text AS library_id,
       l.slug AS library_slug, n.slug AS namespace_slug
FROM library_verifications v
JOIN submissions s ON s.id = v.submission_id
JOIN library_versions lv ON lv.id = v.library_version_id
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE s.user_id = ((sqlc.arg(user_id)::text)::uuid)
ORDER BY v.created_at DESC, v.id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
