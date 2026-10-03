ALTER TABLE problems DROP COLUMN IF EXISTS current_test_cases_key;
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_test_cases_key_check;
ALTER TABLE submissions DROP COLUMN IF EXISTS test_cases_key;
