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

CREATE TABLE libraries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace_id uuid NOT NULL REFERENCES namespaces (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    visibility text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT libraries_namespace_slug_unique UNIQUE (namespace_id, slug),
    CONSTRAINT libraries_slug_check CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT libraries_slug_length_check CHECK (char_length(slug) BETWEEN 2 AND 48),
    CONSTRAINT libraries_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    CONSTRAINT libraries_description_length_check CHECK (char_length(description) <= 2000),
    CONSTRAINT libraries_visibility_check CHECK (visibility IN ('public', 'private'))
);

CREATE INDEX libraries_namespace_visibility_idx
    ON libraries (namespace_id, visibility, created_at DESC, slug ASC);
CREATE INDEX libraries_namespace_created_idx
    ON libraries (namespace_id, created_at DESC, slug ASC);

-- submission_url is an optional, self-declared "related submission" URL the
-- library owner may set, replace, or clear after the version is created. It is
-- never fetched, parsed for content, or judged, and is not proof of a verdict.
--
-- The submission_url CHECK constraints are a defensive DB backstop, NOT the
-- authoritative validator. The authoritative gate is domain.ValidateSubmissionURL,
-- called by the repository before persistence; it trims whitespace, lowercases
-- the scheme, requires a non-empty host via net/url Hostname(), and caps the
-- value at 2048 bytes. absolute_check is anchored to the URL end and excludes
-- '@' from host characters, so hostless authorities ('https://?x',
-- 'https://:443', 'https://user@') are rejected while every URL the domain
-- validator accepts (Unicode/IDN hosts, IPv6 literals, userinfo) still passes.
CREATE TABLE library_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id uuid NOT NULL REFERENCES libraries (id),
    version_number integer NOT NULL,
    source_code text NOT NULL,
    content_sha256 text NOT NULL,
    byte_size integer NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    submission_url text,
    submission_url_updated_at timestamptz,
    CONSTRAINT library_versions_library_number_unique UNIQUE (library_id, version_number),
    CONSTRAINT library_versions_number_check CHECK (version_number >= 1),
    CONSTRAINT library_versions_source_check CHECK (btrim(source_code) <> ''),
    CONSTRAINT library_versions_sha_check CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT library_versions_sha_matches_check CHECK (
        content_sha256 = encode(digest(convert_to(source_code, 'UTF8'), 'sha256'), 'hex')
    ),
    CONSTRAINT library_versions_byte_size_check CHECK (
        byte_size = octet_length(source_code) AND byte_size BETWEEN 1 AND 100000
    ),
    CONSTRAINT library_versions_submission_url_updated_check CHECK (
        (submission_url IS NULL) = (submission_url_updated_at IS NULL)
    ),
    CONSTRAINT library_versions_submission_url_length_check CHECK (octet_length(submission_url) <= 2048),
    CONSTRAINT library_versions_submission_url_absolute_check CHECK (
        submission_url ~ '^https?://([^/?#[:space:][:cntrl:]]*@)?[^/?#:@[:space:][:cntrl:]][^/?#@[:space:][:cntrl:]]*([/?#].*)?$'
    ),
    CONSTRAINT library_versions_submission_url_no_space_check CHECK (submission_url !~ '[[:space:]]'),
    CONSTRAINT library_versions_submission_url_no_control_check CHECK (submission_url !~ '[[:cntrl:]]')
);

CREATE INDEX library_versions_created_by_idx ON library_versions (created_by);

-- A version's code and identity are immutable. Only the self-declared
-- submission URL columns may change; every other UPDATE and every DELETE fails.
CREATE FUNCTION reject_library_version_mutation() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND
       (NEW.id, NEW.library_id, NEW.version_number, NEW.source_code,
        NEW.content_sha256, NEW.byte_size, NEW.created_by, NEW.created_at)
       IS NOT DISTINCT FROM
       (OLD.id, OLD.library_id, OLD.version_number, OLD.source_code,
        OLD.content_sha256, OLD.byte_size, OLD.created_by, OLD.created_at) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'library_versions are immutable';
END;
$$;

CREATE TRIGGER library_versions_no_update_delete
    BEFORE UPDATE OR DELETE ON library_versions
    FOR EACH ROW
    EXECUTE FUNCTION reject_library_version_mutation();
