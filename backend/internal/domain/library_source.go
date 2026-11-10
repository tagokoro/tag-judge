package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// MaxLibrarySourceBytes is the size limit for a library version source.
const MaxLibrarySourceBytes = 100000

var (
	ErrEmptyLibrarySource    = errors.New("source must not be blank")
	ErrNULLibrarySource      = errors.New("source must not contain NUL")
	ErrLibrarySourceTooLarge = errors.New("source must be at most 100000 bytes")
)

// LibrarySourceSHA256 returns the lowercase hex SHA-256 of the raw UTF-8 bytes
// of source. The string is measured and hashed as stored, without trimming.
func LibrarySourceSHA256(source string) string {
	digest := sha256.Sum256([]byte(source))
	return hex.EncodeToString(digest[:])
}

// ValidateLibrarySource enforces the contract for the immutable single-file
// Python source of a library version. The stored string is never trimmed;
// whitespace is only consulted to reject blank sources.
func ValidateLibrarySource(source string) error {
	if strings.TrimSpace(source) == "" {
		return ErrEmptyLibrarySource
	}
	if strings.ContainsRune(source, '\x00') {
		return ErrNULLibrarySource
	}
	if len(source) > MaxLibrarySourceBytes {
		return ErrLibrarySourceTooLarge
	}
	return nil
}
