package auth

import "github.com/tagokoro/tag-judge/backend/internal/domain"

// Method is how a request proved who it acts for.
type Method string

const (
	MethodSession   Method = "session"
	MethodOAuth     Method = "oauth"
	MethodDevHeader Method = "dev_header"
)

type Actor struct {
	UserID string
	Role   domain.UserRole
	// Method is set by the auth middleware; the zero value means an actor
	// built outside a request, such as in a test.
	Method Method
}
