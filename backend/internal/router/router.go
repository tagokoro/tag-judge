package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
)

type Handlers struct {
	Problems        *handler.ProblemHandler
	TestCases       *handler.TestCaseHandler
	CodeRuns        *handler.CodeRunHandler
	Groups          *handler.GroupHandler
	Submissions     *handler.SubmissionHandler
	Me              *handler.MeHandler
	AuthHTTP        *handler.AuthHandler
	Auth            *appmiddleware.AuthMiddleware
	Namespaces      *handler.NamespaceHandler
	Libraries       *handler.LibraryHandler
	LibraryVersions *handler.LibraryVersionHandler
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
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems", handlers.Problems.Create)
	r.Get("/api/v1/problems/{problem_id}", handlers.Problems.Get)
	r.With(handlers.Auth.RequireAuth).Patch("/api/v1/problems/{problem_id}", handlers.Problems.Update)
	r.With(handlers.Auth.RequireAuth).Delete("/api/v1/problems/{problem_id}", handlers.Problems.Delete)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/problems/{problem_id}/test-cases", handlers.TestCases.List)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/problems/{problem_id}/test-cases", handlers.TestCases.Create)
	r.With(handlers.Auth.RequireAuth).Patch("/api/v1/problems/{problem_id}/test-cases/{test_case_id}", handlers.TestCases.Update)
	r.With(handlers.Auth.RequireAuth).Delete("/api/v1/problems/{problem_id}/test-cases/{test_case_id}", handlers.TestCases.Delete)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/code-runs", handlers.CodeRuns.Create)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/code-runs", handlers.CodeRuns.List)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/code-runs/{code_run_id}", handlers.CodeRuns.Get)
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
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/namespaces", handlers.Namespaces.Create)
	r.With(handlers.Auth.RequireAuth).Get("/api/v1/me/namespace", handlers.Namespaces.GetMine)
	r.Get("/api/v1/namespaces/{namespace_slug}", handlers.Namespaces.GetBySlug)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/namespaces/{namespace_slug}/libraries", handlers.Libraries.Create)
	r.Get("/api/v1/namespaces/{namespace_slug}/libraries", handlers.Libraries.List)
	r.Get("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}", handlers.Libraries.Get)
	r.With(handlers.Auth.RequireAuth).Patch("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}", handlers.Libraries.Update)
	r.With(handlers.Auth.RequireAuth).Post("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions", handlers.LibraryVersions.Create)
	r.Get("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions", handlers.LibraryVersions.List)
	r.Get("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions/{version_number}", handlers.LibraryVersions.Get)
	// Only the library owner may set or clear a version's self-declared
	// submission URL; reads come with the version above.
	r.With(handlers.Auth.RequireAuth).Put("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions/{version_number}/submission-url", handlers.LibraryVersions.SetSubmissionURL)
	r.With(handlers.Auth.RequireAuth).Delete("/api/v1/namespaces/{namespace_slug}/libraries/{library_slug}/versions/{version_number}/submission-url", handlers.LibraryVersions.ClearSubmissionURL)

	return r
}
