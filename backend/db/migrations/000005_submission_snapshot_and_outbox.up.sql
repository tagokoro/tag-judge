ALTER TABLE submissions ADD COLUMN test_cases_key text;
UPDATE submissions SET test_cases_key = format('legacy/problems/%s/testcases.json', problem_id) WHERE test_cases_key IS NULL;
ALTER TABLE submissions ALTER COLUMN test_cases_key SET NOT NULL;
ALTER TABLE submissions ADD CONSTRAINT submissions_test_cases_key_check CHECK (btrim(test_cases_key) <> '');

ALTER TABLE submission_results ADD CONSTRAINT submission_results_submission_test_case_unique UNIQUE (submission_id, test_case_id);
