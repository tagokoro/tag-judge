-- The owner's profile row is locked while a library picks its category, so
-- finding or creating the category and the per-user category limit hold under
-- concurrent writes. No row means the user has no handle yet.
-- name: LockUserProfileForCategory :one
SELECT handle
FROM user_profiles
WHERE user_id = ((sqlc.arg(owner_user_id)::text)::uuid)
FOR UPDATE;

-- name: CountCategoriesByOwner :one
SELECT count(*)
FROM categories
WHERE owner_user_id = ((sqlc.arg(owner_user_id)::text)::uuid);

-- Category names are unique per owner ignoring letter case, like the index.
-- name: FindCategoryIDByName :one
SELECT id::text AS id
FROM categories
WHERE owner_user_id = ((sqlc.arg(owner_user_id)::text)::uuid)
  AND lower(name) = lower(sqlc.arg(name));

-- A category starts with only a name; its owner can describe it later.
-- name: CreateCategory :one
INSERT INTO categories (owner_user_id, name)
VALUES (((sqlc.arg(owner_user_id)::text)::uuid), sqlc.arg(name))
RETURNING id::text AS id;

-- has_public_library is read in the same statement as the category, so the
-- "visible to non-owners only with a public library" rule never sees a stale
-- library list.
-- name: GetCategoryByPublicID :one
SELECT c.id::text AS id, c.public_id, c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       c.name, c.description, c.created_at, c.updated_at,
       EXISTS (
           SELECT 1 FROM libraries l WHERE l.category_id = c.id AND l.visibility = 'public'
       ) AS has_public_library
FROM categories c
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE c.public_id = sqlc.arg(public_id);

-- include_hidden lists every category (for the owner); otherwise only
-- categories that contain a public library are listed.
-- name: ListCategoriesByOwner :many
SELECT c.id::text AS id, c.public_id, c.owner_user_id::text AS owner_user_id, up.handle AS owner_handle,
       c.name, c.description, c.created_at, c.updated_at,
       EXISTS (
           SELECT 1 FROM libraries l WHERE l.category_id = c.id AND l.visibility = 'public'
       ) AS has_public_library
FROM categories c
JOIN user_profiles up ON up.user_id = c.owner_user_id
WHERE c.owner_user_id = ((sqlc.arg(owner_user_id)::text)::uuid)
  AND (
      sqlc.arg(include_hidden)::boolean
      OR EXISTS (SELECT 1 FROM libraries l WHERE l.category_id = c.id AND l.visibility = 'public')
  )
ORDER BY c.created_at ASC, c.public_id ASC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: UpdateCategory :one
UPDATE categories
SET name = COALESCE(sqlc.narg(name), name),
    description = COALESCE(sqlc.narg(description), description),
    updated_at = now()
WHERE id = ((sqlc.arg(id)::text)::uuid)
RETURNING id::text AS id, public_id, owner_user_id::text AS owner_user_id, name, description, created_at, updated_at;

-- Removes a category once its last library has left; the emptiness check is
-- part of the DELETE.
-- name: DeleteEmptyCategory :exec
DELETE FROM categories c
WHERE c.id = ((sqlc.arg(id)::text)::uuid)
  AND NOT EXISTS (SELECT 1 FROM libraries l WHERE l.category_id = c.id);
