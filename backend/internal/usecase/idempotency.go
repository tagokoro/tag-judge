package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// canonicalFingerprint returns a stable, compact representation of a value.
// It is used for idempotency replay checks; changing the shape of the input
// produces a different fingerprint and therefore a 409 conflict.
func canonicalFingerprint(v any) (string, error) {
	// json.Marshal on a struct with explicit field tags is deterministic for
	// our purposes because we control the input types and omit maps.
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("canonical fingerprint: %w", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// newIdempotencyKey validates a client-supplied idempotency key and builds the
// key info for admission control. The key is trimmed; a blank key (including
// whitespace-only) is rejected with 400 instead of silently disabling
// idempotency, which could hide client bugs. Length is checked in characters
// (runes) to match the OpenAPI maxLength contract.
func newIdempotencyKey(userID, namespace, rawKey, fingerprint string, maxLen int) (*repository.IdempotencyKeyInfo, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		if rawKey != "" {
			return nil, apperror.InvalidInput("INVALID_IDEMPOTENCY_KEY", "idempotency key must not be blank")
		}
		return nil, nil
	}
	if utf8.RuneCountInString(key) > maxLen {
		return nil, apperror.InvalidInput("INVALID_IDEMPOTENCY_KEY", "idempotency key is too long")
	}
	return &repository.IdempotencyKeyInfo{
		Namespace:   namespace,
		Key:         key,
		Fingerprint: fingerprint,
	}, nil
}
