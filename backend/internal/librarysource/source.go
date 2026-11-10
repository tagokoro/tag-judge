// Package librarysource holds the shared validation and hashing rules for the
// immutable single-file Python source stored on a library version. It performs
// no combining, adapter handling, I/O, HTTP, or database work: with the URL
// link design there is no code to join and no judge run to feed.
package librarysource

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// MaxSourceBytes is the shared size limit for a library version source.
const MaxSourceBytes = 100000

var (
	ErrEmptySource    = errors.New("source must not be blank")
	ErrNULSource      = errors.New("source must not contain NUL")
	ErrSourceTooLarge = errors.New("source must be at most 100000 bytes")
)

// ContentSHA256 returns the lowercase hex SHA-256 of the raw UTF-8 bytes of
// source. The string is measured and hashed as stored, without trimming.
func ContentSHA256(source string) string {
	digest := sha256.Sum256([]byte(source))
	return hex.EncodeToString(digest[:])
}

// ValidateSource enforces the shared archive contract for a library version
// source. The stored string is never trimmed; whitespace is only consulted to
// reject blank sources.
func ValidateSource(source string) error {
	if strings.TrimSpace(source) == "" {
		return ErrEmptySource
	}
	if strings.ContainsRune(source, '\x00') {
		return ErrNULSource
	}
	size := len([]byte(source))
	if size > MaxSourceBytes {
		return ErrSourceTooLarge
	}
	return nil
}
