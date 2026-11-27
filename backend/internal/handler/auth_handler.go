package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const (
	oauthStateCookieName = "tag_judge_oauth_state"
	// loginNextCookieName holds where to return after sign-in, such as the
	// OAuth consent page an MCP client sent the user to.
	loginNextCookieName = "tag_judge_login_next"
)

type AuthHandler struct {
	auth                  *usecase.AuthUsecase
	cookieSecure          bool
	successRedirectURL    string
	onboardingRedirectURL string
}

// onboardingPath is the frontend sign-up page where a new user enters their
// handle.
const onboardingPath = "/signup"

func NewAuthHandler(auth *usecase.AuthUsecase, cookieSecure bool, successRedirectURL string) *AuthHandler {
	return &AuthHandler{
		auth:                  auth,
		cookieSecure:          cookieSecure,
		successRedirectURL:    successRedirectURL,
		onboardingRedirectURL: onboardingRedirectURL(successRedirectURL),
	}
}

// onboardingRedirectURL puts the handle-creation page on the same origin as
// the configured success URL, so no separate setting is needed.
func onboardingRedirectURL(successRedirectURL string) string {
	base, err := url.Parse(successRedirectURL)
	if err != nil {
		return onboardingPath
	}
	return base.ResolveReference(&url.URL{Path: onboardingPath}).String()
}

// safeNextPath accepts only a path on this site, so sign-in cannot be used to
// bounce a user to another site.
func safeNextPath(next string) bool {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		return false
	}
	parsed, err := url.Parse(next)
	return err == nil && parsed.Scheme == "" && parsed.Host == ""
}

// resolveOnOrigin puts path on the origin of base, the configured frontend URL.
func resolveOnOrigin(base, path string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return path
	}
	target, err := parsed.Parse(path)
	if err != nil {
		return path
	}
	return target.String()
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
	if next := r.URL.Query().Get("next"); safeNextPath(next) {
		// Escaped, as a query string may hold bytes a cookie cannot.
		cookie := h.oauthStateCookie(url.QueryEscape(next))
		cookie.Name = loginNextCookieName
		http.SetCookie(w, cookie)
	}
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
	next := ""
	if cookie, err := r.Cookie(loginNextCookieName); err == nil {
		http.SetCookie(w, h.clearCookie(loginNextCookieName))
		if value, err := url.QueryUnescape(cookie.Value); err == nil && safeNextPath(value) {
			next = value
		}
	}
	if session.NeedsHandle {
		target := h.onboardingRedirectURL
		if next != "" {
			// The sign-up page continues to next once the handle exists.
			target += "?next=" + url.QueryEscape(next)
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	if next != "" {
		http.Redirect(w, r, resolveOnOrigin(h.successRedirectURL, next), http.StatusFound)
		return
	}
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
