package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
)

type Handlers struct {
	Problems    *handler.ProblemHandler
	Groups      *handler.GroupHandler
	Submissions *handler.SubmissionHandler
	Me          *handler.MeHandler
	AuthHTTP    *handler.AuthHandler
	Auth        *appmiddleware.AuthMiddleware
}

func New(handlers Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(handlers.Auth.InjectActor)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r.Get("/api/v1/problems", handlers.Problems.List)
	r.Get("/api/v1/problems/{problem_id}", handlers.Problems.Get)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems/{problem_id}/submissions", handlers.Submissions.Create)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/submissions/{submission_id}", handlers.Submissions.Get)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/submissions", handlers.Submissions.List)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/me", handlers.Me.Get)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/groups", handlers.Groups.Create)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/groups", handlers.Groups.List)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/groups/{group_id}", handlers.Groups.Get)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/groups/{group_id}/members", handlers.Groups.AddMember)
	r.Get("/api/v1/auth/google/login", handlers.AuthHTTP.GoogleLogin)
	r.Get("/api/v1/auth/google/callback", handlers.AuthHTTP.GoogleCallback)
	r.Post("/api/v1/auth/logout", handlers.AuthHTTP.Logout)

	return r
}
