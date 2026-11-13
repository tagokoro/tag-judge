package domain

import "time"

// UserProfile is the public side of a user. The handle is their only name: it
// identifies them in URLs and API responses and is what pages show. The Google
// account name and email stay in users and are never part of a profile.
type UserProfile struct {
	UserID    string
	Handle    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ValidHandle validates a user-chosen handle: 2-32 lowercase letters and
// digits joined by single hyphens.
func ValidHandle(handle string) bool {
	return len(handle) >= 2 && len(handle) <= 32 && handlePattern.MatchString(handle)
}
