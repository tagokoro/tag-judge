-- The version number is allocated inside SQL, never in Go. The lock is taken
-- in its own statement so that the later MAX(version_number) read starts with
-- a fresh snapshot after waiting on a concurrent creator.
-- name: LockLibraryByVersion :one
SELECT id
FROM libraries
WHERE id = ((sqlc.arg(library_id)::text)::uuid)
FOR UPDATE;

-- name: InsertLibraryVersion :one
INSERT INTO library_versions (
    library_id,
    version_number,
    source_code,
    content_sha256,
    byte_size,
    created_by
)
SELECT
    ((sqlc.arg(library_id)::text)::uuid),
    COALESCE((
        SELECT MAX(version_number)
        FROM library_versions
        WHERE library_id = ((sqlc.arg(library_id)::text)::uuid)
    ), 0) + 1,
    sqlc.arg(source_code),
    sqlc.arg(content_sha256),
    sqlc.arg(byte_size),
    ((sqlc.arg(created_by)::text)::uuid)
RETURNING
    id::text AS id,
    library_id::text AS library_id,
    version_number,
    source_code,
    content_sha256,
    byte_size,
    created_by::text AS created_by,
    created_at,
    submission_url,
    submission_url_updated_at;

-- Read authorization is enforced IN THE SAME STATEMENT as the data fetch: the
-- join to the current library/namespace lets the WHERE clause gate every row
-- on the library's CURRENT visibility and the actor's user id. A private row
-- is returned only to its owner (no admin/role bypass); an anonymous actor
-- (empty user id) and any non-owner see nothing. Because visibility is read in
-- the same MVCC snapshot as the version body, a concurrent public->private
-- flip cannot expose source that is private at the moment it is returned.
-- name: GetLibraryVersionByNumber :one
SELECT lv.id::text AS id, lv.library_id::text AS library_id, lv.version_number, lv.source_code,
       lv.content_sha256, lv.byte_size, lv.created_by::text AS created_by, lv.created_at,
       lv.submission_url, lv.submission_url_updated_at
FROM library_versions lv
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE lv.library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND lv.version_number = sqlc.arg(version_number)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text));

-- name: ListLibraryVersionSummaries :many
SELECT lv.id::text AS id, lv.library_id::text AS library_id, lv.version_number,
       lv.content_sha256, lv.byte_size, lv.created_by::text AS created_by, lv.created_at,
       lv.submission_url, lv.submission_url_updated_at
FROM library_versions lv
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE lv.library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text))
ORDER BY lv.version_number DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- Only the library owner may set or clear the self-declared submission URL.
-- Ownership is checked in the SAME statement as the write, so no prior read or
-- lock is needed. A NULL submission_url clears both URL columns. No row means
-- the version is missing or the actor is not the owner.
-- name: SetLibraryVersionSubmissionURL :one
UPDATE library_versions lv
SET submission_url = sqlc.narg(submission_url),
    submission_url_updated_at = CASE WHEN sqlc.narg(submission_url)::text IS NULL THEN NULL ELSE now() END
FROM libraries l
JOIN namespaces n ON n.id = l.namespace_id
WHERE l.id = lv.library_id
  AND lv.library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND lv.version_number = sqlc.arg(version_number)
  AND lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text)
RETURNING
    lv.id::text AS id,
    lv.library_id::text AS library_id,
    lv.version_number,
    lv.source_code,
    lv.content_sha256,
    lv.byte_size,
    lv.created_by::text AS created_by,
    lv.created_at,
    lv.submission_url,
    lv.submission_url_updated_at;
