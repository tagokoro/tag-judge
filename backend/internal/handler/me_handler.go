package handler

import (
	"net/http"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type MeHandler struct {
	profiles *usecase.UserProfileUsecase
}

func NewMeHandler(profiles *usecase.UserProfileUsecase) *MeHandler {
	return &MeHandler{profiles: profiles}
}

func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	profile, err := h.profiles.FindMine(r.Context(), actor)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := meResponse{
		UserID: actor.UserID,
		Role:   string(actor.Role),
	}
	if profile != nil {
		response.Handle = &profile.Handle
	}
	writeJSON(w, http.StatusOK, response)
}

// Handle is null until the user creates it.
type meResponse struct {
	UserID string  `json:"user_id"`
	Role   string  `json:"role"`
	Handle *string `json:"handle"`
}
