# Admission Control and Idempotency

This document describes the admission-control behavior implemented for the
Submission and CodeRun create APIs. It covers the current scope only; see
issue #30 for the broader roadmap.

## Scope

- Shared admission controls across `POST /api/v1/problems/{problem_id}/submissions`
  and `POST /api/v1/code-runs`.
- Durable outbox dispatch for CodeRun, mirroring the existing Submission outbox.
- Optional `Idempotency-Key` header for both create APIs.

CodeRun **execution** in the Rust worker is still not implemented; the API only
accepts and durably queues CodeRun jobs.

## Limits

Defaults are configurable via environment variables and must be positive:

| Limit | Default | Env var |
| --- | --- | --- |
| Per-user accepted creation rate | 10 per rolling 60 s | `ADMISSION_USER_RATE_LIMIT` + `ADMISSION_USER_RATE_WINDOW_SECONDS` |
| Per-user pending+running outstanding | 2 | `ADMISSION_USER_OUTSTANDING_LIMIT` |
| Global pending+running outstanding | 100 | `ADMISSION_GLOBAL_OUTSTANDING_LIMIT` |
| Admission lock timeout | 5000 ms | `ADMISSION_LOCK_TIMEOUT_MS` |
| Idempotency key max length | 255 | `IDEMPOTENCY_KEY_MAX_LENGTH` |

Counts include accepted creation requests only. Rejected HTTP attempts do not
add rows and therefore are not counted.

## Serialization

All creation paths acquire a single global lock row
(`admission_control_lock`) in a short `READ COMMITTED` transaction. The lock
is acquired **before** fresh count queries and the execution insert. This makes
admission decisions serializable across multiple API processes without needing
per-user locks.

S3/SQS/network operations are never performed inside the locked transaction:

- Submissions store source code before the admission transaction.
- CodeRuns reserve an ID outside the locked transaction, store the source
  artifact using the reserved ID, and then pass that exact key into the
  atomic CodeRun+outbox creation. Storage failure before admission creates
  neither a CodeRun nor an outbox row. A rejected admission decision or an
  idempotent replay of an existing key may leave a pre-stored artifact
  without a referencing CodeRun; such unreferenced artifacts are retained
  until a future cleanup job removes them.

## HTTP Behavior

The existing JSON error envelope is preserved. Status codes:

| Condition | Status | Error Code | Retry-After |
| --- | --- | --- | --- |
| Per-user rate limit | 429 | `USER_RATE_LIMIT_EXCEEDED` | seconds until oldest request leaves the window |
| Per-user outstanding limit | 429 | `USER_OUTSTANDING_LIMIT_EXCEEDED` | 1 (estimate) |
| Global outstanding limit | 503 | `GLOBAL_OUTSTANDING_LIMIT_EXCEEDED` | 5 (estimate) |
| DB lock timeout / unavailable | 503 | `TRANSIENT_DATABASE_ERROR` | 1 |
| Changed idempotency payload | 409 | `IDEMPOTENCY_KEY_CONFLICT` | none |
| Unexpected internal failure | 500 | `INTERNAL_ERROR` | none |

HTTP success is returned only after the transaction commits. If the client
loses the response, retrying with the same `Idempotency-Key` replays the same
committed record instead of creating a duplicate.

## Idempotency Keys

- Keys are optional. Without a key, the API makes no duplicate-suppression
  guarantee on retry.
- A key is scoped to the authenticated user and an operation namespace
  (`submissions:create` or `code-runs:create`).
- The server stores a fingerprint of the normalized request body. Replaying the
  same key with the same fingerprint returns the existing execution. Replaying
  the same key with a different fingerprint returns `409`.
- Keys are retained indefinitely in the current implementation. No retention
  cleanup is configured yet; premature deletion could allow unwanted duplicates.
- Keys are validated for length and are never logged.

## CodeRun Outbox

CodeRuns now use `code_run_job_outbox`, with a publisher started alongside the
Submission publisher. The publisher retries SQS dispatch with exponential
backoff and marks records dispatched only after `SendMessage` succeeds. This
preserves Submission payload compatibility and retry behavior.

## Tests

PostgreSQL integration tests live in
`backend/internal/infra/postgres/admission_integration_test.go`. They use an
isolated schema per test run and never truncate shared data. Set
`ADMISSION_TEST_DATABASE_URL` to run them.

Coverage includes:

- Exact rate-limit boundaries and the rolling window.
- Combined outstanding counts across submissions and code runs.
- Global and per-user limit interactions.
- Rejected calls leaving no execution or outbox rows.
- CodeRun outbox dispatch, retry, and payload compatibility.
- Idempotent replay, changed-payload conflict, concurrent same-key calls, and
  replay when admission limits are full.
- Concurrent creates observing exactly one per-user outstanding slot and exactly
  one global outstanding slot across different users.
- Injected transaction failure rolling back execution, outbox, and idempotency
  rows together.
- Cascade deletion of idempotency keys when the referenced submission or code
  run is deleted.
- CodeRun source-code pre-storage: the outbox payload uses the pre-stored key
  and storage failure creates no CodeRun or outbox row.
