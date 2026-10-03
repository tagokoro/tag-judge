ALTER TABLE submission_results
    DROP CONSTRAINT IF EXISTS submission_results_submission_test_case_unique;

DROP TABLE IF EXISTS submission_job_outbox;
