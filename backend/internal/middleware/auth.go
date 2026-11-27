package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
)

const (
	userIDHeader   = "X-User-ID"
	userRoleHeader = "X-User-Role"
)

type SessionStore interface {
	FindActorBySessionTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error)
}

type AccessTokenStore interface {
	FindActorByAccessTokenHash(ctx context.Context, tokenHash string) (auth.Actor, error)
}

type AuthMiddleware struct {
	sessions      SessionStore
	tokens        AccessTokenStore
	devHeaderAuth bool
	// resourceMetadataURL, when set, is advertised in Bearer challenges so an
	// MCP client can find the authorization server (RFC 9728).
	resourceMetadataURL string
}

func NewAuthMiddleware(sessions SessionStore, tokens AccessTokenStore, devHeaderAuth bool) *AuthMiddleware {
	return &AuthMiddleware{
		sessions:      sessions,
		tokens:        tokens,
		devHeaderAuth: devHeaderAuth,
	}
}

// WithResourceMetadata sets the URL Bearer challenges point clients to.
func (m *AuthMiddleware) WithResourceMetadata(url string) *AuthMiddleware {
	m.resourceMetadataURL = url
	return m
}

func (m *AuthMiddleware) InjectActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.sessions != nil {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err == nil && cookie.Value != "" {
				actor, err := m.sessions.FindActorBySessionTokenHash(r.Context(), auth.HashToken(cookie.Value))
				if err == nil {
					actor.Method = auth.MethodSession
					ctx := auth.ContextWithActor(r.Context(), actor)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		if !m.devHeaderAuth {
			next.ServeHTTP(w, r)
			return
		}

		userID := r.Header.Get(userIDHeader)
		role := domain.UserRole(r.Header.Get(userRoleHeader))
		if userID == "" && role == "" {
			next.ServeHTTP(w, r)
			return
		}
		if userID == "" || !role.Valid() {
			handler.WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
			return
		}

		ctx := auth.ContextWithActor(r.Context(), auth.Actor{
			UserID: userID,
			Role:   role,
			Method: auth.MethodDevHeader,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.ActorFromContext(r.Context()); !ok {
			handler.WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireToken guards /mcp, the only route that accepts OAuth access tokens:
// a token grants what the MCP tools offer, not the whole REST API. Browser
// sessions are refused, so a page open in the browser cannot drive the MCP
// tools with the user's cookie, and the 401 tells an MCP client where to
// start OAuth.
func (m *AuthMiddleware) RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := bearerToken(r); ok && m.tokens != nil {
			actor, err := m.tokens.FindActorByAccessTokenHash(r.Context(), auth.HashToken(token))
			if errors.Is(err, apperror.ErrUnauthorized) {
				m.challenge(w, `error="invalid_token"`)
				handler.WriteError(w, r, err)
				return
			}
			if err != nil {
				// A failed lookup says nothing about the token, so the client
				// must not be told to authenticate again.
				handler.WriteError(w, r, err)
				return
			}
			actor.Method = auth.MethodOAuth
			next.ServeHTTP(w, r.WithContext(auth.ContextWithActor(r.Context(), actor)))
			return
		}

		actor, ok := auth.ActorFromContext(r.Context())
		if !ok || actor.Method != auth.MethodDevHeader {
			m.challenge(w, "")
			handler.WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) challenge(w http.ResponseWriter, params string) {
	if m.resourceMetadataURL != "" {
		if params != "" {
			params += ", "
		}
		params += `resource_metadata="` + m.resourceMetadataURL + `"`
	}
	value := "Bearer"
	if params != "" {
		value += " " + params
	}
	w.Header().Set("WWW-Authenticate", value)
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
