DROP INDEX IF EXISTS code_runs_status_pending_running_idx;
DROP INDEX IF EXISTS code_runs_user_id_status_idx;
DROP INDEX IF EXISTS submissions_status_pending_running_idx;
DROP INDEX IF EXISTS submissions_user_id_status_idx;
DROP TABLE IF EXISTS execution_idempotency_keys;
DROP TABLE IF EXISTS admission_control_lock;
