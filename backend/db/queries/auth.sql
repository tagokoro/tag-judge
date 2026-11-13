-- Signs a Google user in: inserts them on their first login, and otherwise
-- updates name and email only when Google reports a change, so updated_at
-- keeps meaning "the data changed", not "they logged in". An unchanged row is
-- not returned by the upsert, so the existing row is read instead. Both parts
-- read the same snapshot: a first login racing another first login of the
-- same user can come back empty, and the caller retries once.
--
-- has_handle tells the login callback whether the user still has to register
-- a handle. user_profiles.handle is NOT NULL, so a profile row means a
-- registered handle; a required profile field added later must be checked
-- here too.
-- name: UpsertGoogleUser :one
WITH upserted AS (
    INSERT INTO users (provider, provider_user_id, name, email, role)
    VALUES ('google', sqlc.arg(provider_user_id), sqlc.arg(name), sqlc.arg(email), 'user')
    ON CONFLICT (provider, provider_user_id) DO UPDATE
    SET
        name = EXCLUDED.name,
        email = EXCLUDED.email,
        updated_at = now()
    WHERE (users.name, users.email) IS DISTINCT FROM (EXCLUDED.name, EXCLUDED.email)
    RETURNING id, role
),
u AS (
    SELECT id, role FROM upserted
    UNION ALL
    SELECT id, role
    FROM users
    WHERE provider = 'google'
      AND provider_user_id = sqlc.arg(provider_user_id)
      AND NOT EXISTS (SELECT 1 FROM upserted)
)
SELECT u.id::text AS user_id, u.role,
       EXISTS (SELECT 1 FROM user_profiles p WHERE p.user_id = u.id) AS has_handle
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
