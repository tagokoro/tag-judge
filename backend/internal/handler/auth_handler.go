package handler

import (
	"net/http"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const oauthStateCookieName = "tag_judge_oauth_state"

type AuthHandler struct {
	auth               *usecase.AuthUsecase
	cookieSecure       bool
	successRedirectURL string
}

func NewAuthHandler(auth *usecase.AuthUsecase, cookieSecure bool, successRedirectURL string) *AuthHandler {
	return &AuthHandler{
		auth:               auth,
		cookieSecure:       cookieSecure,
		successRedirectURL: successRedirectURL,
	}
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := auth.NewToken()
	if err != nil {
		WriteError(w, r, err)
		return
	}

	loginURL, err := h.auth.GoogleLoginURL(state)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	http.SetCookie(w, h.oauthStateCookie(state))
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || state == "" || stateCookie.Value != state {
		WriteError(w, r, apperror.InvalidInput("INVALID_OAUTH_STATE", "oauth state is invalid"))
		return
	}

	session, err := h.auth.HandleGoogleCallback(r.Context(), code)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	http.SetCookie(w, h.clearCookie(oauthStateCookieName))
	http.SetCookie(w, h.sessionCookie(session))
	http.Redirect(w, r, h.successRedirectURL, http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		if err := h.auth.Logout(r.Context(), cookie.Value); err != nil {
			WriteError(w, r, err)
			return
		}
	}

	http.SetCookie(w, h.clearCookie(auth.SessionCookieName))
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) oauthStateCookie(state string) *http.Cookie {
	return &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *AuthHandler) sessionCookie(session auth.Session) *http.Cookie {
	return &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *AuthHandler) clearCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}
