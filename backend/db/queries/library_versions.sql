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
SELECT id::text AS id, library_id::text AS library_id, version_number, source_code,
       content_sha256, byte_size, created_by::text AS created_by, created_at
FROM library_versions
WHERE library_id = ((sqlc.arg(library_id)::text)::uuid)
  AND version_number = sqlc.arg(version_number);

-- name: ListLibraryVersionSummaries :many
SELECT id::text AS id, library_id::text AS library_id, version_number,
       content_sha256, byte_size, created_by::text AS created_by, created_at
FROM library_versions
WHERE library_id = ((sqlc.arg(library_id)::text)::uuid)
ORDER BY version_number DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
