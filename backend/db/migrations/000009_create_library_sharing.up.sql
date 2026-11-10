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

CREATE TABLE library_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id uuid NOT NULL REFERENCES libraries (id),
    version_number integer NOT NULL,
    source_code text NOT NULL,
    content_sha256 text NOT NULL,
    byte_size integer NOT NULL,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT library_versions_library_number_unique UNIQUE (library_id, version_number),
    CONSTRAINT library_versions_number_check CHECK (version_number >= 1),
    CONSTRAINT library_versions_source_check CHECK (btrim(source_code) <> ''),
    CONSTRAINT library_versions_sha_check CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT library_versions_sha_matches_check CHECK (
        content_sha256 = encode(digest(convert_to(source_code, 'UTF8'), 'sha256'), 'hex')
    ),
    CONSTRAINT library_versions_byte_size_check CHECK (
        byte_size = octet_length(source_code) AND byte_size BETWEEN 1 AND 100000
    )
);

CREATE INDEX library_versions_created_by_idx ON library_versions (created_by);

CREATE FUNCTION reject_library_version_mutation() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'library_versions are immutable';
END;
$$;

CREATE TRIGGER library_versions_no_update_delete
    BEFORE UPDATE OR DELETE ON library_versions
    FOR EACH ROW
    EXECUTE FUNCTION reject_library_version_mutation();

-- library_verifications stores self-declared "related submission" links for a
-- library version. The table name is kept from the earlier design to keep the
-- PR review diff small, but a row is only a plain absolute HTTP(S) submission
-- URL plus its registrant and timestamp. There is NO adapter, harness,
-- Submission, Outbox, Judge run, verdict, score, or case result. The URL is
-- never fetched, parsed for content, or judged; a link is self-declared and is
-- not verified proof of the submitter, verdict, or the library version used.
--
-- These CHECK constraints are a defensive DB backstop, NOT the authoritative
-- validator. The authoritative gate is domain.ValidateSubmissionURL, called by
-- the repository before persistence (so invalid URLs are rejected even outside
-- the future API layer); it trims whitespace, requires a non-empty host via
-- net/url, and caps the value at 2048 bytes.
--
-- absolute_check must require a REAL host, not just "something after //". The
-- earlier '^https?://[^/[:space:][:cntrl:]]+' pattern wrongly accepted hostless
-- authorities such as 'https://?x', 'https://@' and 'https://:443' because '?',
-- '@' and ':' were allowed as the first authority character. This pattern
-- requires the authority (after any optional userinfo ending in @) to begin
-- with a host character: an ASCII letter/digit, or '[' for an IPv6 literal.
-- It is anchored to the end (with an optional /, ? or # path/query/fragment
-- tail) so a bare userinfo or empty-port authority cannot masquerade as a host.
CREATE TABLE library_verifications (
    id bigserial PRIMARY KEY,
    library_version_id uuid NOT NULL REFERENCES library_versions (id),
    created_by uuid NOT NULL REFERENCES users (id),
    submission_url text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT library_verifications_submission_url_length_check CHECK (octet_length(submission_url) <= 2048),
    CONSTRAINT library_verifications_submission_url_absolute_check CHECK (
        submission_url ~ '^https?://([^@/?#[:space:][:cntrl:]]*@)?[A-Za-z0-9\[][^/@?#[:space:][:cntrl:]]*([/?#].*)?$'
    ),
    CONSTRAINT library_verifications_submission_url_no_control_check CHECK (submission_url !~ '[[:cntrl:]]')
);

CREATE INDEX library_verifications_version_idx
    ON library_verifications (library_version_id);
CREATE INDEX library_verifications_created_idx
    ON library_verifications (created_at DESC, id DESC);
