DROP INDEX IF EXISTS submissions_public_created_at_idx;
ALTER TABLE submissions DROP COLUMN IF EXISTS is_public;
