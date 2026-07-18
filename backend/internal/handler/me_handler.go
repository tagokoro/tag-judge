package handler

import (
	"net/http"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
)

type MeHandler struct{}

func NewMeHandler() *MeHandler {
	return &MeHandler{}
}

func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	writeJSON(w, http.StatusOK, meResponse{
		UserID: actor.UserID,
		Role:   string(actor.Role),
	})
}

type meResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
