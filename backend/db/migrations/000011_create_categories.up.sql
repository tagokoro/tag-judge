-- Libraries are filed under categories (such as "グラフ") that a user names
-- freely and that are created and removed with the libraries in them, so
-- nobody manages them by hand. Categories and libraries are identified by a
-- random public id, like Qiita's /{user}/items/{id}, so nobody has to invent a
-- URL slug. Nothing has been released yet, so the namespace data is dropped
-- rather than migrated.
TRUNCATE library_versions, libraries, namespaces;

ALTER TABLE libraries DROP CONSTRAINT libraries_namespace_id_fkey;
DROP TABLE namespaces;

-- A category belongs to a profile, so a user without a handle cannot own one.
-- Its name is unique per owner ignoring letter case. The internal uuid stays
-- the key every foreign key uses; only public_id leaves the API.
CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id text NOT NULL DEFAULT encode(gen_random_bytes(10), 'hex'),
    owner_user_id uuid NOT NULL REFERENCES user_profiles (user_id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_public_id_unique UNIQUE (public_id),
    CONSTRAINT categories_public_id_check CHECK (public_id ~ '^[0-9a-f]{20}$'),
    CONSTRAINT categories_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 64),
    CONSTRAINT categories_description_length_check CHECK (char_length(description) <= 500)
);

CREATE UNIQUE INDEX categories_owner_name_unique ON categories (owner_user_id, lower(name));
CREATE INDEX categories_owner_created_idx ON categories (owner_user_id, created_at, public_id);

-- A library is named freely (names may repeat) and found by its public id, so
-- moving it to another category keeps its URL. The old list indexes sort by
-- slug, so they go before the column does.
DROP INDEX libraries_namespace_visibility_idx;
DROP INDEX libraries_namespace_created_idx;
ALTER TABLE libraries
    DROP CONSTRAINT libraries_namespace_slug_unique,
    DROP CONSTRAINT libraries_slug_check,
    DROP CONSTRAINT libraries_slug_length_check,
    DROP COLUMN slug,
    ADD COLUMN public_id text NOT NULL DEFAULT encode(gen_random_bytes(10), 'hex'),
    ADD CONSTRAINT libraries_public_id_unique UNIQUE (public_id),
    ADD CONSTRAINT libraries_public_id_check CHECK (public_id ~ '^[0-9a-f]{20}$');
ALTER TABLE libraries RENAME COLUMN namespace_id TO category_id;
ALTER TABLE libraries
    ADD CONSTRAINT libraries_category_id_fkey FOREIGN KEY (category_id) REFERENCES categories (id);

-- Library lists are ordered by the last update.
CREATE INDEX libraries_category_visibility_updated_idx
    ON libraries (category_id, visibility, updated_at DESC, public_id ASC);
CREATE INDEX libraries_category_updated_idx
    ON libraries (category_id, updated_at DESC, public_id ASC);
