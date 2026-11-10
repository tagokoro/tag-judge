-- The create transaction re-checks whom the library belongs to and whether the
-- version still belongs to the library using this single locked snapshot.
-- libraries is locked with FOR SHARE so a concurrent visibility PATCH waits,
-- preventing a non-owner from committing a link while the library is privatized.
-- name: LockLibraryForSubmissionLink :one
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

-- A link stores only the version, the registrant, and the submission URL.
-- name: InsertLibrarySubmissionLink :one
INSERT INTO library_submission_links (library_version_id, created_by, submission_url)
VALUES (((sqlc.arg(library_version_id)::text)::uuid), ((sqlc.arg(created_by)::text)::uuid), sqlc.arg(submission_url))
RETURNING id;

-- The visibility predicate is evaluated in the SAME statement as link data.
-- Public links are readable by anyone; private links only by the owner.
-- No submission, problem, status, score, or adapter columns are read.
-- name: GetLibrarySubmissionLink :one
SELECT v.id, v.library_version_id::text AS library_version_id, v.submission_url,
       v.created_by::text AS created_by, v.created_at,
       lv.version_number, lv.library_id::text AS library_id,
       l.slug AS library_slug, l.visibility,
       n.slug AS namespace_slug, n.owner_user_id::text AS owner_user_id
FROM library_submission_links v
JOIN library_versions lv ON lv.id = v.library_version_id
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE v.id = sqlc.arg(submission_link_id)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text));

-- name: ListLibrarySubmissionLinksByVersion :many
SELECT v.id, v.library_version_id::text AS library_version_id, v.submission_url,
       v.created_by::text AS created_by, v.created_at,
       lv.version_number, lv.library_id::text AS library_id,
       l.slug AS library_slug, l.visibility,
       n.slug AS namespace_slug, n.owner_user_id::text AS owner_user_id
FROM library_submission_links v
JOIN library_versions lv ON lv.id = v.library_version_id
JOIN libraries l ON l.id = lv.library_id
JOIN namespaces n ON n.id = l.namespace_id
WHERE v.library_version_id = ((sqlc.arg(library_version_id)::text)::uuid)
  AND (l.visibility = 'public' OR lower(n.owner_user_id::text) = lower(sqlc.arg(actor_user_id)::text))
ORDER BY v.created_at DESC, v.id DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
