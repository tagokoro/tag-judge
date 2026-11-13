package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type UserProfileHandler struct {
	profiles *usecase.UserProfileUsecase
}

func NewUserProfileHandler(profiles *usecase.UserProfileUsecase) *UserProfileHandler {
	return &UserProfileHandler{profiles: profiles}
}

// CreateMine creates the caller's handle (POST /me/profile).
func (h *UserProfileHandler) CreateMine(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request struct {
		Handle string `json:"handle"`
	}
	if err := decodeJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	profile, err := h.profiles.CreateMine(r.Context(), actor, request.Handle)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newUserProfileResponse(*profile))
}

func (h *UserProfileHandler) GetByHandle(w http.ResponseWriter, r *http.Request) {
	profile, err := h.profiles.GetByHandle(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newUserProfileResponse(*profile))
}

// userProfileResponse is the public profile: never the user id, email, or
// Google name.
type userProfileResponse struct {
	Handle string `json:"handle"`
}

func newUserProfileResponse(profile domain.UserProfile) userProfileResponse {
	return userProfileResponse{Handle: profile.Handle}
}
