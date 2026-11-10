-- name: CreateLibrary :one
INSERT INTO libraries (namespace_id, slug, name, description, visibility)
VALUES (((sqlc.arg(namespace_id)::text)::uuid), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(description), sqlc.arg(visibility))
RETURNING id, namespace_id::text AS namespace_id, slug, name, description, visibility, created_at, updated_at;

-- name: UpdateLibrary :one
UPDATE libraries
SET name = COALESCE(sqlc.narg(name), name),
    description = COALESCE(sqlc.narg(description), description),
    visibility = COALESCE(sqlc.narg(visibility), visibility),
    updated_at = now()
WHERE id = ((sqlc.arg(id)::text)::uuid)
RETURNING id, namespace_id::text AS namespace_id, slug, name, description, visibility, created_at, updated_at;

-- name: GetLibraryBySlug :one
SELECT id, namespace_id::text AS namespace_id, slug, name, description, visibility, created_at, updated_at
FROM libraries
WHERE namespace_id = ((sqlc.arg(namespace_id)::text)::uuid)
  AND slug = sqlc.arg(slug);

-- name: ListPublicLibraries :many
SELECT id, namespace_id::text AS namespace_id, slug, name, description, visibility, created_at, updated_at
FROM libraries
WHERE namespace_id = ((sqlc.arg(namespace_id)::text)::uuid)
  AND visibility = 'public'
ORDER BY created_at DESC, slug ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: ListNamespaceLibraries :many
SELECT id, namespace_id::text AS namespace_id, slug, name, description, visibility, created_at, updated_at
FROM libraries
WHERE namespace_id = ((sqlc.arg(namespace_id)::text)::uuid)
ORDER BY created_at DESC, slug ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- Public libraries across every namespace, newest first, for the site-wide
-- library list. Private libraries never appear here.
-- name: ListAllPublicLibraries :many
SELECT l.id, n.slug AS namespace_slug, l.slug, l.name, l.description, l.visibility, l.created_at, l.updated_at
FROM libraries l
JOIN namespaces n ON n.id = l.namespace_id
WHERE l.visibility = 'public'
ORDER BY l.created_at DESC, l.id ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
