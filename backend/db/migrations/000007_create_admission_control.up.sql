-- Admission control uses a single global lock row so that counts and inserts
-- are serialized across API processes. A global lock is sufficient because
-- every admission decision reads fresh counts before inserting.
CREATE TABLE admission_control_lock (
    id integer PRIMARY KEY CHECK (id = 1)
);

INSERT INTO admission_control_lock (id) VALUES (1);

-- Idempotency keys associate a client-supplied key with the execution record
-- created for a specific request body. The fingerprint prevents replaying a
-- key with different inputs. Keys are retained indefinitely; a cleanup job
-- can be added later.
CREATE TABLE execution_idempotency_keys (
    id bigserial PRIMARY KEY,
    user_id uuid NOT NULL,
    namespace text NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    submission_id bigint REFERENCES submissions(id) ON DELETE CASCADE,
    code_run_id bigint REFERENCES code_runs(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT execution_idempotency_keys_user_namespace_key_unique UNIQUE (user_id, namespace, key),
    CONSTRAINT execution_idempotency_keys_exactly_one_execution CHECK (
        (submission_id IS NOT NULL AND code_run_id IS NULL)
        OR (submission_id IS NULL AND code_run_id IS NOT NULL)
    )
);

-- Partial indexes for fast pending+running outstanding counts.
CREATE INDEX submissions_user_id_status_idx
    ON submissions (user_id, status)
    WHERE status IN ('pending', 'running');

CREATE INDEX submissions_status_pending_running_idx
    ON submissions (status)
    WHERE status IN ('pending', 'running');

CREATE INDEX code_runs_user_id_status_idx
    ON code_runs (user_id, status)
    WHERE status IN ('pending', 'running');

CREATE INDEX code_runs_status_pending_running_idx
    ON code_runs (status)
    WHERE status IN ('pending', 'running');
