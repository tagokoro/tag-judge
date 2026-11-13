-- Going back to one slug-named namespace per user cannot keep categories or
-- libraries without a slug, so the library data is dropped again. The
-- restored namespaces reference users, as in 000009.
TRUNCATE library_versions, libraries, categories;

DROP INDEX libraries_category_updated_idx;
DROP INDEX libraries_category_visibility_updated_idx;
ALTER TABLE libraries DROP CONSTRAINT libraries_category_id_fkey;
ALTER TABLE libraries RENAME COLUMN category_id TO namespace_id;
ALTER TABLE libraries
    DROP CONSTRAINT libraries_public_id_check,
    DROP CONSTRAINT libraries_public_id_unique,
    DROP COLUMN public_id,
    ADD COLUMN slug text NOT NULL,
    ADD CONSTRAINT libraries_namespace_slug_unique UNIQUE (namespace_id, slug),
    ADD CONSTRAINT libraries_slug_check CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    ADD CONSTRAINT libraries_slug_length_check CHECK (char_length(slug) BETWEEN 2 AND 48);

CREATE INDEX libraries_namespace_visibility_idx
    ON libraries (namespace_id, visibility, created_at DESC, slug ASC);
CREATE INDEX libraries_namespace_created_idx
    ON libraries (namespace_id, created_at DESC, slug ASC);

DROP TABLE categories;

CREATE TABLE namespaces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users (id),
    slug text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT namespaces_owner_user_unique UNIQUE (owner_user_id),
    CONSTRAINT namespaces_slug_unique UNIQUE (slug),
    CONSTRAINT namespaces_slug_check CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT namespaces_slug_length_check CHECK (char_length(slug) BETWEEN 2 AND 32),
    CONSTRAINT namespaces_display_name_check CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 64)
);

ALTER TABLE libraries
    ADD CONSTRAINT libraries_namespace_id_fkey FOREIGN KEY (namespace_id) REFERENCES namespaces (id);
