-- name: CreateOAuthClient :exec
INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris)
VALUES (sqlc.arg(id), sqlc.narg(secret_hash), sqlc.arg(name), sqlc.arg(redirect_uris)::text[]);

-- name: GetOAuthClient :one
SELECT id, secret_hash, name, redirect_uris, created_at
FROM oauth_clients
WHERE id = $1;

-- name: CreateOAuthAuthorizationCode :exec
INSERT INTO oauth_authorization_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, expires_at)
VALUES (sqlc.arg(code_hash), sqlc.arg(client_id), sqlc.arg(user_id)::text::uuid, sqlc.arg(redirect_uri), sqlc.arg(code_challenge), sqlc.arg(expires_at));

-- A code is marked used as it is read, so it can be exchanged only once.
-- name: ConsumeOAuthAuthorizationCode :one
UPDATE oauth_authorization_codes
SET used_at = now()
WHERE code_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
RETURNING client_id, user_id::text AS user_id, redirect_uri, code_challenge;

-- name: CreateOAuthToken :exec
INSERT INTO oauth_tokens (client_id, user_id, access_token_hash, access_expires_at, refresh_token_hash, refresh_expires_at)
VALUES (sqlc.arg(client_id), sqlc.arg(user_id)::text::uuid, sqlc.arg(access_token_hash), sqlc.arg(access_expires_at), sqlc.arg(refresh_token_hash), sqlc.arg(refresh_expires_at));

-- name: RevokeOAuthTokenByRefreshHash :one
UPDATE oauth_tokens
SET revoked_at = now()
WHERE refresh_token_hash = sqlc.arg(refresh_token_hash)
  AND client_id = sqlc.arg(client_id)
  AND revoked_at IS NULL
  AND refresh_expires_at > now()
RETURNING user_id::text AS user_id;

-- Resolving a token also records when it was last used.
-- name: UseOAuthAccessToken :one
UPDATE oauth_tokens
SET last_used_at = now()
FROM users
WHERE oauth_tokens.access_token_hash = $1
  AND oauth_tokens.revoked_at IS NULL
  AND oauth_tokens.access_expires_at > now()
  AND users.id = oauth_tokens.user_id
RETURNING users.id::text AS user_id, users.role;
