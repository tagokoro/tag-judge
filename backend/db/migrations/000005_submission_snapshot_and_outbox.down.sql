ALTER TABLE submission_results DROP CONSTRAINT IF EXISTS submission_results_submission_test_case_unique;
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_test_cases_key_check;
ALTER TABLE submissions DROP COLUMN IF EXISTS test_cases_key;
