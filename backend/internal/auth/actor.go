package auth

import "github.com/tagokoro/tag-judge/backend/internal/domain"

type Actor struct {
	UserID string
	Role   domain.UserRole
}
