package usecase

import (
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
)

func validateActor(actor auth.Actor) error {
	if !validUUID(actor.UserID) || !actor.Role.Valid() {
		return apperror.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	return nil
}
