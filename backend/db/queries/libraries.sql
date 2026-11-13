-- Every library query returns the same row: the library with its category
-- and owner, so a library is always shown and authorized with its location.

-- name: CreateLibrary :one
WITH l AS (
    INSERT INTO libraries (category_id, name, description, visibility)
    VALUES (((sqlc.arg(category_id)::text)::uuid), sqlc.arg(name), sqlc.arg(description), sqlc.arg(visibility))
    RETURNING *
)
SELECT l.id::text AS id, l.public_id, c.public_id AS category_public_id, c.name AS category_name,
       c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       l.name, l.description, l.visibility, l.created_at, l.updated_at
FROM l
JOIN categories c ON c.id = l.category_id
JOIN user_profiles up ON up.user_id = c.owner_user_id;

-- The library's current category, locked for a move.
-- name: LockLibraryCategory :one
SELECT category_id::text AS category_id
FROM libraries
WHERE id = ((sqlc.arg(id)::text)::uuid)
FOR UPDATE;

-- category_id (internal) moves the library; the caller resolved it among the
-- owner's categories. The join reads the category the library ends up in, so
-- the returned location is the new one.
-- name: UpdateLibrary :one
UPDATE libraries l
SET name = COALESCE(sqlc.narg(name), l.name),
    description = COALESCE(sqlc.narg(description), l.description),
    visibility = COALESCE(sqlc.narg(visibility), l.visibility),
    category_id = COALESCE((sqlc.narg(category_id)::text)::uuid, l.category_id),
    updated_at = now()
FROM categories c
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE l.id = ((sqlc.arg(id)::text)::uuid)
  AND c.id = COALESCE((sqlc.narg(category_id)::text)::uuid, l.category_id)
RETURNING l.id::text AS id, l.public_id, c.public_id AS category_public_id, c.name AS category_name,
          c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
          l.name, l.description, l.visibility, l.created_at, l.updated_at;

-- name: GetLibraryByPublicID :one
SELECT l.id::text AS id, l.public_id, c.public_id AS category_public_id, c.name AS category_name,
       c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       l.name, l.description, l.visibility, l.created_at, l.updated_at
FROM libraries l
JOIN categories c ON c.id = l.category_id
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE l.public_id = sqlc.arg(public_id);

-- include_private lists every library (for the owner); otherwise only public
-- ones. Newest update first.
-- name: ListCategoryLibraries :many
SELECT l.id::text AS id, l.public_id, c.public_id AS category_public_id, c.name AS category_name,
       c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       l.name, l.description, l.visibility, l.created_at, l.updated_at
FROM libraries l
JOIN categories c ON c.id = l.category_id
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE l.category_id = ((sqlc.arg(category_id)::text)::uuid)
  AND (sqlc.arg(include_private)::boolean OR l.visibility = 'public')
ORDER BY l.updated_at DESC, l.public_id ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- Public libraries across every user, newest first, for the site-wide
-- library list. Private libraries never appear here.
-- name: ListAllPublicLibraries :many
SELECT l.id::text AS id, l.public_id, c.public_id AS category_public_id, c.name AS category_name,
       c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       l.name, l.description, l.visibility, l.created_at, l.updated_at
FROM libraries l
JOIN categories c ON c.id = l.category_id
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE l.visibility = 'public'
ORDER BY l.created_at DESC, l.public_id ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);
