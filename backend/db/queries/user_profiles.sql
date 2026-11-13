-- name: CreateUserProfile :one
INSERT INTO user_profiles (user_id, handle)
VALUES (((sqlc.arg(user_id)::text)::uuid), sqlc.arg(handle))
RETURNING user_id::text AS user_id, handle, created_at, updated_at;

-- name: GetUserProfileByUserID :one
SELECT user_id::text AS user_id, handle, created_at, updated_at
FROM user_profiles
WHERE user_id = ((sqlc.arg(user_id)::text)::uuid);

-- name: GetUserProfileByHandle :one
SELECT user_id::text AS user_id, handle, created_at, updated_at
FROM user_profiles
WHERE handle = sqlc.arg(handle);
