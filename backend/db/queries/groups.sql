-- name: CreateGroup :one
INSERT INTO groups (name, description, created_by)
VALUES (sqlc.arg(name), sqlc.arg(description), sqlc.arg(created_by)::text::uuid)
RETURNING id, name, description, created_by::text AS created_by, created_at, updated_at;

-- name: AddGroupMember :one
INSERT INTO group_members (group_id, user_id, role)
VALUES (sqlc.arg(group_id), sqlc.arg(user_id)::text::uuid, sqlc.arg(role))
RETURNING group_id, user_id::text AS user_id, role, created_at;

-- name: GetGroup :one
SELECT id, name, description, created_by::text AS created_by, created_at, updated_at
FROM groups
WHERE id = $1;

-- name: ListGroupsByUserID :many
SELECT g.id, g.name, g.description, g.created_by::text AS created_by, g.created_at, g.updated_at
FROM groups AS g
INNER JOIN group_members AS gm ON gm.group_id = g.id
WHERE gm.user_id = sqlc.arg(user_id)::text::uuid
ORDER BY g.id DESC;

-- name: GetGroupMember :one
SELECT group_id, user_id::text AS user_id, role, created_at
FROM group_members
WHERE group_id = sqlc.arg(group_id) AND user_id = sqlc.arg(user_id)::text::uuid;
