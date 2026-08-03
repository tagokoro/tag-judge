package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
)

type Handlers struct {
	Problems *handler.ProblemHandler
	Me       *handler.MeHandler
	AuthHTTP *handler.AuthHandler
	Auth     *appmiddleware.AuthMiddleware
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
	r.Get("/api/v1/problems/{problem_id}", handlers.Problems.GetVisible)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems", handlers.Problems.Create)
	r.With(handlers.Auth.RequireAuth).Put("/api/v1/problems/{problem_id}", handlers.Problems.Update)
	r.With(handlers.Auth.RequireAuth).Delete("/api/v1/problems/{problem_id}", handlers.Problems.Delete)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/problems/{problem_id}/test-cases", handlers.Problems.ListTestCases)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems/{problem_id}/test-cases", handlers.Problems.CreateTestCase)
	r.With(handlers.Auth.RequireAuth).Put("/api/v1/problems/{problem_id}/test-cases/{test_case_id}", handlers.Problems.UpdateTestCase)
	r.With(handlers.Auth.RequireAuth).Delete("/api/v1/problems/{problem_id}/test-cases/{test_case_id}", handlers.Problems.DeleteTestCase)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/problems/{problem_id}/submissions", handlers.Problems.ListSubmissions)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems/{problem_id}/submissions", handlers.Problems.CreateSubmission)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/submissions/{submission_id}", handlers.Problems.GetSubmission)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/me", handlers.Me.Get)
	r.Get("/api/v1/auth/google/login", handlers.AuthHTTP.GoogleLogin)
	r.Get("/api/v1/auth/google/callback", handlers.AuthHTTP.GoogleCallback)
	r.Post("/api/v1/auth/logout", handlers.AuthHTTP.Logout)

	return r
}
