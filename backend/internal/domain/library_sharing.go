package domain

import (
	"regexp"
	"time"
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

// LibraryVerification records that one library version was verified against
// one existing public problem. The problem and the starter stay with the
// submissions row; no duplicated columns exist here.
type LibraryVerification struct {
	ID               int64
	LibraryVersionID string
	LibrarySlug      string
	NamespaceSlug    string
	VersionNumber    int
	ProblemID        int64
	SubmissionID     int64
	UserID           string
	Language         SubmissionLanguage
	Status           SubmissionStatus
	Score            int
	AdapterSource    string
	AdapterSHA256    string
	CreatedAt        time.Time
	JudgedAt         *time.Time
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
