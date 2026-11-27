-- Tag Judge is an OAuth 2.1 authorization server for MCP clients such as
-- Claude. A client registers itself (RFC 7591), the signed-in user approves
-- it, and it receives an access token for /mcp. Codes and tokens are stored
-- only as SHA-256 hashes, like sessions.
CREATE TABLE oauth_clients (
    id text PRIMARY KEY,
    -- secret_hash is NULL for a public client, which proves itself with PKCE
    -- alone.
    secret_hash text,
    name text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT oauth_clients_redirect_uris_check CHECK (cardinality(redirect_uris) BETWEEN 1 AND 10)
);

CREATE TABLE oauth_authorization_codes (
    code_hash text PRIMARY KEY,
    client_id text NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Each row is one access/refresh token pair. Refreshing revokes the row and
-- inserts a new one, so a refresh token works only once.
CREATE TABLE oauth_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id text NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token_hash text NOT NULL UNIQUE,
    access_expires_at timestamptz NOT NULL,
    refresh_token_hash text NOT NULL UNIQUE,
    refresh_expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at timestamptz
);

CREATE INDEX oauth_tokens_user_id_idx ON oauth_tokens (user_id, created_at DESC);
