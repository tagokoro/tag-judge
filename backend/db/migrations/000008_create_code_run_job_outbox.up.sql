-- CodeRun execution jobs are persisted in the DB before dispatch, matching the
-- Submission outbox pattern. Accepted code runs cannot be lost because of a
-- temporary SQS failure.
CREATE TABLE code_run_job_outbox (
    code_run_id bigint PRIMARY KEY REFERENCES code_runs(id) ON DELETE CASCADE,
    payload jsonb NOT NULL,
    dispatched_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT code_run_job_outbox_attempts_check CHECK (attempts >= 0)
);

CREATE INDEX code_run_job_outbox_pending_idx
    ON code_run_job_outbox (available_at, code_run_id)
    WHERE dispatched_at IS NULL;
