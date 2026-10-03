-- A submission and its work item are inserted in one DB transaction. This
-- closes the gap between accepting a submission and publishing it to SQS.
CREATE TABLE submission_job_outbox (
    submission_id bigint PRIMARY KEY REFERENCES submissions(id) ON DELETE CASCADE,
    payload jsonb NOT NULL,
    dispatched_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT submission_job_outbox_attempts_check CHECK (attempts >= 0)
);

CREATE INDEX submission_job_outbox_pending_idx
    ON submission_job_outbox (available_at, submission_id)
    WHERE dispatched_at IS NULL;

ALTER TABLE submission_results
    ADD CONSTRAINT submission_results_submission_test_case_unique
    UNIQUE (submission_id, test_case_id);
