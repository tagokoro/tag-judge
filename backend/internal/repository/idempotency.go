package repository

const (
	IdempotencyNamespaceSubmission = "submissions:create"
	IdempotencyNamespaceCodeRun    = "code-runs:create"
)

// IdempotencyKeyInfo identifies a create request for safe retry. The Key is
// supplied by the client; the Fingerprint is a stable representation of the
// normalized request body. Replays match only when both the key and the
// fingerprint match. The key must never be logged.
type IdempotencyKeyInfo struct {
	Namespace   string
	Key         string
	Fingerprint string
}
