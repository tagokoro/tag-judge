-- Whether anyone may read a submission (source included) is decided once, when
-- it is accepted: a problem can only move from private/group to public, so a
-- submission made while the problem was a draft stays private for good.
-- Existing rows take the problem's current visibility; the false default
-- keeps a forgotten value on the private side.
ALTER TABLE submissions ADD COLUMN is_public boolean NOT NULL DEFAULT false;

UPDATE submissions s
SET is_public = true
FROM problems p
WHERE p.id = s.problem_id AND p.visibility = 'public';

CREATE INDEX submissions_public_created_at_idx ON submissions (created_at DESC, id DESC) WHERE is_public;
