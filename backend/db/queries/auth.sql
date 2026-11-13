-- has_profile tells the login callback whether the user still has to pick a
-- handle.
-- name: UpsertGoogleUser :one
WITH u AS (
    INSERT INTO users (provider, provider_user_id, name, email, role)
    VALUES ('google', sqlc.arg(provider_user_id), sqlc.arg(name), sqlc.arg(email), 'user')
    ON CONFLICT (provider, provider_user_id) DO UPDATE
    SET
        name = EXCLUDED.name,
        email = EXCLUDED.email,
        updated_at = now()
    RETURNING id, role
)
SELECT u.id::text AS user_id, u.role,
       EXISTS (SELECT 1 FROM user_profiles p WHERE p.user_id = u.id) AS has_profile
FROM u;

-- name: CreateSession :exec
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(token_hash), sqlc.arg(expires_at));

-- name: GetActorBySessionTokenHash :one
SELECT users.id::text AS user_id, users.role
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = $1
  AND sessions.revoked_at IS NULL
  AND sessions.expires_at > now();

-- name: RevokeSessionByTokenHash :exec
UPDATE sessions
SET revoked_at = now()
WHERE token_hash = $1
  AND revoked_at IS NULL;
