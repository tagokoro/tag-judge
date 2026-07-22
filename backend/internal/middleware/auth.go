package middleware

import (
	"net/http"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
)

const (
	userIDHeader   = "X-User-ID"
	userRoleHeader = "X-User-Role"
)

type AuthMiddleware struct{}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

func (m *AuthMiddleware) InjectActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
