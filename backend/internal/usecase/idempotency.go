package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// canonicalFingerprint returns a stable, compact representation of a value.
// It is used for idempotency replay checks; changing the shape of the input
// produces a different fingerprint and therefore a 409 conflict.
func canonicalFingerprint(v any) string {
	// json.Marshal on a struct with explicit field tags is deterministic for
	// our purposes because we control the input types and omit maps.
	b, err := json.Marshal(v)
	if err != nil {
		// json.Marshal on plain structs cannot fail in practice.
		b = []byte{}
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
