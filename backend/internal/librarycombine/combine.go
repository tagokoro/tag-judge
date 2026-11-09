// Package librarycombine joins a library version source and a per-problem
// adapter source into one Python submission accepted by the existing judge.
// It performs no I/O and knows nothing about HTTP, storage, or the database.
package librarycombine

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// MaxSourceBytes is the shared size limit for library and adapter sources.
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

// ValidateSource enforces the shared archive contract for library and adapter
// sources. The stored string is never trimmed; whitespace is only consulted to
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

// Combine validates both sources and returns the single Python source the
// existing judge executes as a normal submission. The library runs under
// __name__ == "library", so its top-level __main__ block stays inert, and the
// adapter runs under " __main__" sharing the library's globals.
func Combine(librarySource, adapterSource string) (string, error) {
	if err := ValidateSource(librarySource); err != nil {
		return "", fmt.Errorf("invalid library source: %w", err)
	}
	if err := ValidateSource(adapterSource); err != nil {
		return "", fmt.Errorf("invalid adapter source: %w", err)
	}

	encoding := base64.StdEncoding.EncodeToString([]byte(librarySource))
	adapterEncoding := base64.StdEncoding.EncodeToString([]byte(adapterSource))

	var builder strings.Builder
	builder.WriteString("import base64\n")
	builder.WriteString("_library = base64.b64decode('" + encoding + "').decode(\"utf-8\")\n")
	builder.WriteString("_adapter = base64.b64decode('" + adapterEncoding + "').decode(\"utf-8\")\n")
	builder.WriteString("_namespace = {\"__name__\": \"library\"}\n")
	builder.WriteString("exec(compile(_library, \"<library>\", \"exec\"), _namespace)\n")
	builder.WriteString("_namespace[\"__name__\"] = \"__main__\"\n")
	builder.WriteString("exec(compile(_adapter, \"<adapter>\", \"exec\"), _namespace)\n")
	return builder.String(), nil
}
