package domain

import "time"

// OAuthClient is an application, such as Claude, registered to ask users for
// access to Tag Judge.
type OAuthClient struct {
	ID string
	// SecretHash is empty for a public client.
	SecretHash   string
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

// OAuthAuthorizationCode is what a user's approval stands on until the client
// exchanges it for tokens.
type OAuthAuthorizationCode struct {
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
}
