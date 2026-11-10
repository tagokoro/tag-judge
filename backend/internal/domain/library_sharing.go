package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// NamespaceSlugPattern is the shared slug contract for namespaces and
// libraries: lowercase letters and digits joined by single hyphens.
var namespaceSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// LibraryVisibility is the read access level of a library. Only public and
// private exist; group and unlisted are outside this version.
type LibraryVisibility string

const (
	LibraryVisibilityPublic  LibraryVisibility = "public"
	LibraryVisibilityPrivate LibraryVisibility = "private"
)

func (v LibraryVisibility) Valid() bool {
	return v == LibraryVisibilityPublic || v == LibraryVisibilityPrivate
}

// Library is a named set of code kept under a personal namespace. Visibility
// decides who can read it; writes stay with the namespace owner.
type Library struct {
	ID            string
	NamespaceID   string
	NamespaceSlug string
	Slug          string
	Name          string
	Description   string
	Visibility    LibraryVisibility
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// LibraryVersion is one immutable snapshot of a library's single-file Python
// source. Version numbers are assigned by the database starting at 1.
type LibraryVersion struct {
	ID            string
	LibraryID     string
	VersionNumber int
	SourceCode    string
	ContentSHA256 string
	ByteSize      int
	CreatedBy     string
	CreatedAt     time.Time
}

// LibraryVerification is a self-declared "related submission" link for one
// library version: a plain absolute HTTP(S) submission URL. The DB table name
// is kept from the earlier design to keep the PR review diff small, but a row
// is NOT a verified proof: there is no adapter, harness, Submission, Outbox,
// Judge run, verdict, score, or case result. The URL is never fetched, parsed
// for content, or judged; it merely points at a submission the registrant says
// is related. Authenticity of the submitter, verdict, or the library version
// used is not guaranteed.
//
// Read methods join the current library/namespace so callers can authorize
// reads by the library's CURRENT visibility (public links are viewable by
// anyone, private links by the owner only). Those fields are read-time context,
// not columns stored on the link itself.
type LibraryVerification struct {
	ID               int64
	LibraryVersionID string
	SubmissionURL    string
	CreatedBy        string
	CreatedAt        time.Time
	// Current library state, joined at read time for authorization:
	NamespaceSlug string
	LibrarySlug   string
	VersionNumber int
	Visibility    LibraryVisibility
	OwnerUserID   string
}

// Namespace is the single personal space a user owns. It holds libraries and
// has no group membership in this version.
type Namespace struct {
	ID          string
	OwnerUserID string
	Slug        string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidNamespaceSlug validates a user-created namespace slug (2-32 chars).
func ValidNamespaceSlug(slug string) bool {
	return validSlug(slug, 2, 32)
}

// ValidLibrarySlug validates a user-created library slug (2-48 chars).
func ValidLibrarySlug(slug string) bool {
	return validSlug(slug, 2, 48)
}

func validSlug(slug string, min, max int) bool {
	return len(slug) >= min && len(slug) <= max && namespaceSlugPattern.MatchString(slug)
}

// MaxSubmissionURLLength is the maximum accepted length (in bytes) of a
// submission URL. It matches the library_verifications DB CHECK backstop.
const MaxSubmissionURLLength = 2048

// ErrInvalidSubmissionURL is returned by ValidateSubmissionURL when the value
// is not a plain absolute HTTP(S) URL with a non-empty host.
var ErrInvalidSubmissionURL = errors.New("invalid submission URL")

func isHostStartByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') ||
		b == '[' // IPv6 literal in brackets
}

// ValidateSubmissionURL is the authoritative FORMAT gate for a self-declared
// submission link. It trims surrounding whitespace and requires an absolute
// HTTP(S) URL with a real (non-empty) host, no control characters or internal
// whitespace, and at most MaxSubmissionURLLength bytes. It never fetches the
// URL, parses its content, or invokes the judge; it only checks the string's
// shape. The returned value is the trimmed URL to persist.
//
// This is the authoritative validator, not a comment or a DB CHECK: the
// repository calls it before any write, so invalid URLs (including hostless
// authorities such as "https://?x", "https://@" or "https://:443") are rejected
// even when Create is called directly, outside the future API layer. The
// isHostStartByte/authority check mirrors the DB backstop so that no URL
// accepted here can be rejected at INSERT time.
func ValidateSubmissionURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > MaxSubmissionURLLength {
		return "", ErrInvalidSubmissionURL
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrInvalidSubmissionURL
		}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", ErrInvalidSubmissionURL
	}
	// Hostname() strips an optional port, so ":443" (Host=":443") and "?x"/"@"
	// (Host="") all yield an empty hostname and are rejected as hostless.
	// Opaque is non-empty for "scheme:foo" forms without "//".
	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Opaque != "" || parsed.Hostname() == "" {
		return "", ErrInvalidSubmissionURL
	}
	// Mirror the DB CHECK: the authority (what follows "://") must start with an
	// ASCII host character or '[' (IPv6). This keeps the Go gate a strict
	// superset of the DB backstop.
	idx := strings.Index(value, "://")
	if idx < 0 {
		return "", ErrInvalidSubmissionURL
	}
	authority := value[idx+3:]
	if authority == "" || !isHostStartByte(authority[0]) {
		return "", ErrInvalidSubmissionURL
	}
	return value, nil
}
