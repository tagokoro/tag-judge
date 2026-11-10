-- name: CreateNamespace :one
INSERT INTO namespaces (owner_user_id, slug, display_name)
VALUES (((sqlc.arg(owner_user_id)::text)::uuid), sqlc.arg(slug), sqlc.arg(display_name))
RETURNING id, owner_user_id::text AS owner_user_id, slug, display_name, created_at, updated_at;

-- name: GetNamespaceByOwner :one
SELECT id, owner_user_id::text AS owner_user_id, slug, display_name, created_at, updated_at
FROM namespaces
WHERE owner_user_id = ((sqlc.arg(owner_user_id)::text)::uuid);

-- name: GetNamespaceBySlug :one
SELECT id, owner_user_id::text AS owner_user_id, slug, display_name, created_at, updated_at
FROM namespaces
WHERE slug = $1;
