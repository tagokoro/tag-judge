package auth

import "time"

type Session struct {
	Token     string
	ExpiresAt time.Time
	// NeedsHandle is true when the user has not created their handle yet.
	NeedsHandle bool
}
