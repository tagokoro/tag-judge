-- Users get a public handle, their only name, chosen once on the sign-up page.
-- users keeps the login data (Google name, email), which no public query
-- reads; user_profiles is the public side.
CREATE TABLE user_profiles (
    user_id uuid PRIMARY KEY REFERENCES users (id),
    handle text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_profiles_handle_unique UNIQUE (handle),
    CONSTRAINT user_profiles_handle_check CHECK (handle ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT user_profiles_handle_length_check CHECK (char_length(handle) BETWEEN 2 AND 32)
);
