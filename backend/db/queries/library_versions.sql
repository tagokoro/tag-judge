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
    created_at;

-- name: GetLibraryVersionByNumber :one
SELECT lv.id::text AS id, lv.library_id::text AS library_id, lv.version_number, lv.source_code,
       lv.content_sha256, lv.byte_size, lv.created_by::text AS created_by, lv.created_at
FROM library_versions lv
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE lv.library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND lv.version_number = sqlc.arg(version_number)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text));

-- name: ListLibraryVersionSummaries :many
SELECT lv.id::text AS id, lv.library_id::text AS library_id, lv.version_number,
       lv.content_sha256, lv.byte_size, lv.created_by::text AS created_by, lv.created_at
FROM library_versions lv
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE lv.library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text))
ORDER BY lv.version_number DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
