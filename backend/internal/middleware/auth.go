package middleware

import (
	"context"
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
		// A Bearer token is checked before the session cookie: a request that
		// carries one asks to act as that token, and /mcp accepts only tokens.
		if token, ok := bearerToken(r); ok && m.tokens != nil {
			actor, err := m.tokens.FindActorByAccessTokenHash(r.Context(), auth.HashToken(token))
			if err != nil {
				// A bad token is rejected rather than treated as anonymous, so
				// the client learns it must authenticate again.
				m.challenge(w, `error="invalid_token"`)
				handler.WriteError(w, r, apperror.Unauthorized("INVALID_TOKEN", "access token is invalid, expired, or revoked"))
				return
			}
			actor.Method = auth.MethodOAuth
			ctx := auth.ContextWithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

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

// RequireToken guards /mcp. Browser sessions are refused, so a page open in
// the browser cannot drive the MCP tools with the user's cookie, and the 401
// tells an MCP client where to start OAuth.
func (m *AuthMiddleware) RequireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.ActorFromContext(r.Context())
		if !ok || actor.Method == auth.MethodSession {
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
